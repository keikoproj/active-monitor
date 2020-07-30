/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	cron "github.com/robfig/cron/v3"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	activemonitorv1alpha1 "github.com/keikoproj/active-monitor/api/v1alpha1"
	"github.com/keikoproj/active-monitor/metrics"
	"github.com/keikoproj/active-monitor/store"
)

const (
	hcKind                    = "HealthCheck"
	hcVersion                 = "v1alpha1"
	wfGroup                   = "argoproj.io"
	wfVersion                 = "v1alpha1"
	wfKind                    = "Workflow"
	wfResource                = "workflows"
	succStr                   = "Succeeded"
	failStr                   = "Failed"
	defaultWorkflowTTLSec     = 1800
	remedy                    = "remedy"
	healthcheck               = "healthCheck"
	healthCheckClusterLevel   = "cluster"
	healthCheckNamespaceLevel = "namespace"
	WfInstanceIdLabelKey      = "workflows.argoproj.io/controller-instanceid"
	WfInstanceId              = "activemonitor-workflows"
)

var (
	wfGvk = schema.GroupVersionKind{
		Group:   wfGroup,
		Version: wfVersion,
		Kind:    wfKind,
	}
	wfGvr = schema.GroupVersionResource{
		Group:    wfGroup,
		Version:  wfVersion,
		Resource: wfResource,
	}
)

// HealthCheckReconciler reconciles a HealthCheck object
type HealthCheckReconciler struct {
	client.Client
	DynClient          dynamic.Interface
	kubeclient         *kubernetes.Clientset
	Log                logr.Logger
	RepeatTimersByName map[string]*time.Timer
	workflowLabels     map[string]string
}

func ignoreNotFound(err error) error {
	if apierrs.IsNotFound(err) {
		return nil
	}
	return err
}

// +kubebuilder:rbac:groups=activemonitor.keikoproj.io,resources=healthchecks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=activemonitor.keikoproj.io,resources=healthchecks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=argoproj.io,resources=workflow;workflows,verbs=get;list;watch;create;update;patch;delete

// Reconcile per kubebuilder v2 pattern
func (r *HealthCheckReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues(hcKind, req.NamespacedName)
	log.Info("Starting HealthCheck reconcile for ...")

	// initialize timers map if not already done
	if r.RepeatTimersByName == nil {
		r.RepeatTimersByName = make(map[string]*time.Timer)
	}

	var healthCheck = &activemonitorv1alpha1.HealthCheck{}
	if err := r.Get(ctx, req.NamespacedName, healthCheck); err != nil {
		// if our healthcheck was deleted, this Reconcile method is invoked with an empty resource cache
		// see: https://book.kubebuilder.io/cronjob-tutorial/controller-implementation.html#1-load-the-cronjob-by-name
		log.Info("Healthcheck object not found for reconciliation, likely deleted")
		// stop timer corresponding to next schedule run of workflow since parent healthcheck no longer exists
		if r.RepeatTimersByName[req.NamespacedName.Name] != nil {
			log.Info("Cancelling rescheduled workflow for this healthcheck due to deletion")
			r.RepeatTimersByName[req.NamespacedName.Name].Stop()
		}
		return ctrl.Result{}, ignoreNotFound(err)
	}

	return r.processOrRecoverHealthCheck(ctx, log, healthCheck)
}

func (r *HealthCheckReconciler) processOrRecoverHealthCheck(ctx context.Context, log logr.Logger, healthCheck *activemonitorv1alpha1.HealthCheck) (ctrl.Result, error) {
	defer func() {
		if err := recover(); err != nil {
			log.Info("Error: Panic occurred during execAdd %s/%s due to %s", healthCheck.Name, healthCheck.Namespace, err)
		}
	}()
	// Process HealthCheck
	ret, procErr := r.processHealthCheck(ctx, log, healthCheck)

	err := r.Update(ctx, healthCheck)
	if err != nil {
		log.Error(err, "Error updating healthcheck resource")
		// Force retry when status fails to update
		return ctrl.Result{}, err
	}

	return ret, procErr
}

func (r *HealthCheckReconciler) processHealthCheck(ctx context.Context, log logr.Logger, healthCheck *activemonitorv1alpha1.HealthCheck) (ctrl.Result, error) {

	hcSpec := healthCheck.Spec
	if hcSpec.Workflow.Resource != nil {
		wfNamePrefix := hcSpec.Workflow.GenerateName
		wfNamespace := hcSpec.Workflow.Resource.Namespace
		now := metav1.Time{Time: time.Now()}
		var finishedAtTime int64
		if healthCheck.Status.FinishedAt != nil {
			finishedAtTime = healthCheck.Status.FinishedAt.Time.Unix()
		}

		// workflows can be paused by setting repeatAfterSec to <= 0 and not specifying the schedule for cron.
		if hcSpec.RepeatAfterSec <= 0 && hcSpec.Schedule.Cron == "" {
			log.Info("Workflow will be skipped due to repeatAfterSec value", "repeatAfterSec", hcSpec.RepeatAfterSec)
			healthCheck.Status.Status = "Stopped"
			healthCheck.Status.ErrorMessage = fmt.Sprintf("workflow execution is stopped; either spec.RepeatAfterSec or spec.Schedule must be provided. spec.RepeatAfterSec set to %d. spec.Schedule set to %+v", hcSpec.RepeatAfterSec, hcSpec.Schedule)
			healthCheck.Status.FinishedAt = &now
			return ctrl.Result{}, nil
		} else if hcSpec.RepeatAfterSec <= 0 && hcSpec.Schedule.Cron != "" {
			log.Info("Workflow to be set with Schedule", "Cron", hcSpec.Schedule.Cron)
			schedule, err := cron.ParseStandard(hcSpec.Schedule.Cron)
			if err != nil {
				log.Error(err, "fail to parse cron")
			}
			// The value from schedule next and substracting from current time is in fraction as we convert to int it will be 1 less than
			// the intended reschedule so we need to add 1sec to get the actual value
			// we need to update the spec so have to healthCheck.Spec.RepeatAfterSec instead of local variable hcSpec
			healthCheck.Spec.RepeatAfterSec = int(schedule.Next(time.Now()).Sub(time.Now())/time.Second) + 1
			log.Info("spec.RepeatAfterSec value is set", "RepeatAfterSec", healthCheck.Spec.RepeatAfterSec)
		} else if int(time.Now().Unix()-finishedAtTime) < hcSpec.RepeatAfterSec {
			log.Info("Workflow already executed", "finishedAtTime", finishedAtTime)
			return ctrl.Result{}, nil
		}

		err := r.createRBACForWorkflow(log, healthCheck, hcKind)
		if err != nil {
			log.Error(err, "Error creating RBAC for HealthCheckWorkflow")
			return ctrl.Result{}, err
		}

		log.Info("Creating Workflow", "namespace", wfNamespace, "generateNamePrefix", wfNamePrefix)
		generatedWfName, err := r.createSubmitWorkflow(ctx, log, healthCheck)
		if err != nil {
			log.Error(err, "Error creating or submitting workflow")
			return ctrl.Result{}, err
		}
		err = r.watchWorkflowReschedule(ctx, ctrl.Request{}, log, wfNamespace, generatedWfName, healthCheck)
		if err != nil {
			log.Error(err, "Error executing Workflow")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager as used in main package by kubebuilder v2.0.0.alpha4
func (r *HealthCheckReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.kubeclient = kubernetes.NewForConfigOrDie(mgr.GetConfig())
	return ctrl.NewControllerManagedBy(mgr).
		For(&activemonitorv1alpha1.HealthCheck{}).
		Complete(r)
}

func (r *HealthCheckReconciler) createRBACForWorkflow(log logr.Logger, hc *activemonitorv1alpha1.HealthCheck, workFlowType string) error {
	level := hc.Spec.Level
	hcSa := hc.Spec.Workflow.Resource.ServiceAccount
	wfNamespace := hc.Spec.Workflow.Resource.Namespace
	amclusterRole := hcSa + "-cluster-role"
	amclusterRoleBinding := hcSa + "-cluster-role-binding"
	amnsRole := hcSa + "-ns-role"
	amnsRoleBinding := hcSa + "-ns-role-binding"
	var remedySa, amclusterRemedyRole, amclusterRoleRemedyBinding, amnsRemedyRole, amnsRemedyRoleBinding, wfRemedyNamespace string
	if !hc.Spec.RemedyWorkflow.IsEmpty() {
		if hc.Spec.RemedyWorkflow.Resource.ServiceAccount != "" {
			if hcSa == hc.Spec.RemedyWorkflow.Resource.ServiceAccount {
				hc.Spec.RemedyWorkflow.Resource.ServiceAccount = hcSa + "-remedy"
			}
			remedySa = hc.Spec.RemedyWorkflow.Resource.ServiceAccount
			amclusterRemedyRole = remedySa + "-cluster-role"
			amclusterRoleRemedyBinding = remedySa + "-cluster-role-binding"
			amnsRemedyRole = remedySa + "-ns-role"
			amnsRemedyRoleBinding = remedySa + "-ns-role-binding"
			wfRemedyNamespace = hc.Spec.RemedyWorkflow.Resource.Namespace
		} else {
			return errors.New("ServiceAccount for the RemedyWorkflow is not specified")
		}
	}

	if workFlowType != remedy {
		servacc, err := r.createServiceAccount(r.kubeclient, hcSa, wfNamespace)
		if err != nil {
			log.Error(err, "Error creating ServiceAccount for the workflow")
			return err
		}
		log.Info("Successfully Created", "ServiceAccount", servacc)
	} else {
		servacc1, err := r.createServiceAccount(r.kubeclient, remedySa, wfRemedyNamespace)
		if err != nil {
			log.Error(err, "Error creating ServiceAccount for the workflow")
			return err
		}
		log.Info("Successfully Created", "ServiceAccount", servacc1)
	}
	if level == healthCheckClusterLevel {

		if workFlowType != remedy {
			clusrole, err := r.createClusterRole(r.kubeclient, amclusterRole)
			if err != nil {
				log.Error(err, "Error creating ClusterRole for the healthcheck workflow")
				return err
			}
			log.Info("Successfully Created", "ClusterRole", clusrole)

			crb, err := r.createClusterRoleBinding(r.kubeclient, amclusterRoleBinding, amclusterRole, hcSa, wfNamespace)
			if err != nil {
				log.Error(err, "Error creating ClusterRoleBinding for the workflow")
				return err
			}
			log.Info("Successfully Created", "ClusterRoleBinding", crb)

		} else {
			clusrole1, err := r.createRemedyClusterRole(r.kubeclient, amclusterRemedyRole)
			if err != nil {
				log.Error(err, "Error creating ClusterRole for the remedy workflow")
				return err
			}
			log.Info("Successfully Created", "ClusterRole", clusrole1)

			crb1, err := r.createClusterRoleBinding(r.kubeclient, amclusterRoleRemedyBinding, amclusterRemedyRole, remedySa, wfRemedyNamespace)
			if err != nil {
				log.Error(err, "Error creating ClusterRoleBinding for the workflow")
				return err
			}
			log.Info("Successfully Created", "ClusterRoleBinding", crb1)

		}

	} else if level == healthCheckNamespaceLevel {

		if workFlowType != remedy {
			nsRole, err := r.createNameSpaceRole(r.kubeclient, amnsRole, wfNamespace)
			if err != nil {
				log.Error(err, "Error creating NamespaceRole for the workflow")
				return err
			}
			log.Info("Successfully Created", "NamespaceRole", nsRole)

			nsrb, err := r.createNameSpaceRoleBinding(r.kubeclient, amnsRoleBinding, amnsRole, hcSa, wfNamespace)
			if err != nil {
				log.Error(err, "Error creating NamespaceRoleBinding for the workflow")
				return err
			}
			log.Info("Successfully Created", "NamespaceRoleBinding", nsrb)

		} else {
			nsRole1, err := r.createRemedyNameSpaceRole(r.kubeclient, amnsRemedyRole, wfRemedyNamespace)
			if err != nil {
				log.Error(err, "Error creating NamespaceRole for the workflow")
				return err
			}
			log.Info("Successfully Created", "NamespaceRole", nsRole1)

			nsrb1, err := r.createNameSpaceRoleBinding(r.kubeclient, amnsRemedyRoleBinding, amnsRemedyRole, remedySa, wfRemedyNamespace)
			if err != nil {
				log.Error(err, "Error creating NamespaceRoleBinding for the workflow")
				return err
			}
			log.Info("Successfully Created", "NamespaceRoleBinding", nsrb1)

		}

	} else {
		return errors.New("level is not set")
	}

	return nil
}

func (r *HealthCheckReconciler) deleteRBACForWorkflow(log logr.Logger, hc *activemonitorv1alpha1.HealthCheck) error {
	level := hc.Spec.Level
	remedySa := hc.Spec.RemedyWorkflow.Resource.ServiceAccount
	wfRemedyNamespace := hc.Spec.RemedyWorkflow.Resource.Namespace

	amclusterRemedyRole := remedySa + "-cluster-role"
	amclusterRoleRemedyBinding := remedySa + "-cluster-role-binding"

	amnsRemedyRole := remedySa + "-ns-role"
	amnsRemedyRoleBinding := remedySa + "-ns-role-binding"
	err := r.DeleteServiceAccount(r.kubeclient, remedySa, wfRemedyNamespace)
	if err != nil {
		log.Error(err, "Error deleting ServiceAccount for the workflow")
		return err
	}
	log.Info("Successfully Deleted", "ServiceAccount", remedySa)

	if level == "cluster" {

		err = r.DeleteClusterRole(r.kubeclient, amclusterRemedyRole)
		if err != nil {
			log.Error(err, "Error creating ClusterRole for the workflow")
			return err
		}
		log.Info("Successfully Deleted", "ClusterRole", amclusterRemedyRole)

		err = r.DeleteClusterRoleBinding(r.kubeclient, amclusterRoleRemedyBinding, amclusterRemedyRole, remedySa, wfRemedyNamespace)
		if err != nil {
			log.Error(err, "Error creating ClusterRoleBinding for the workflow")
			return err
		}
		log.Info("Successfully Created", "ClusterRoleBinding", amclusterRoleRemedyBinding)

	} else if level == "namespace" {

		err := r.DeleteNameSpaceRole(r.kubeclient, amnsRemedyRole, wfRemedyNamespace)
		if err != nil {
			log.Error(err, "Error creating NamespaceRole for the workflow")
			return err
		}
		log.Info("Successfully Deleted", "NamespaceRole", amnsRemedyRole)

		err = r.DeleteNameSpaceRoleBinding(r.kubeclient, amnsRemedyRoleBinding, amnsRemedyRole, remedySa, wfRemedyNamespace)
		if err != nil {
			log.Error(err, "Error creating NamespaceRoleBinding for the workflow")
			return err
		}
		log.Info("Successfully Deleted", "NamespaceRoleBinding", amnsRemedyRoleBinding)

	} else {
		err := errors.New("level is not set")
		return err
	}

	return nil
}

// this function exists to assist with how a function called by the timer.AfterFunc() method operates to call a
// function which takes parameters, it is easiest to build this closure which holds access to the parameters we need.
// the helper returns a function object taking no parameters directly, this is what we want to give AfterFunc
func (r *HealthCheckReconciler) createSubmitWorkflowHelper(ctx context.Context, log logr.Logger, wfNamespace string, hc *activemonitorv1alpha1.HealthCheck) func() {
	return func() {
		log.Info("Creating and Submitting Workflow...")
		wfName, err := r.createSubmitWorkflow(ctx, log, hc)
		if err != nil {
			log.Error(err, "Error creating or submitting workflow")
		}
		err = r.watchWorkflowReschedule(ctx, ctrl.Request{}, log, wfNamespace, wfName, hc)
		if err != nil {
			log.Error(err, "Error watching or rescheduling workflow")
		}
	}
}

func (r *HealthCheckReconciler) createSubmitWorkflow(ctx context.Context, log logr.Logger, hc *activemonitorv1alpha1.HealthCheck) (wfName string, err error) {
	workflow := &unstructured.Unstructured{}
	r.parseWorkflowFromHealthcheck(log, hc, workflow)
	workflow.SetGroupVersionKind(wfGvk)
	workflow.SetNamespace(hc.Spec.Workflow.Resource.Namespace)
	workflow.SetGenerateName(hc.Spec.Workflow.GenerateName)
	workflow.SetLabels(r.workflowLabels)
	// set the owner references for workflow
	ownerReferences := workflow.GetOwnerReferences()
	trueVar := true
	newRef := metav1.OwnerReference{
		Kind:       hcKind,
		APIVersion: hcVersion,
		Name:       hc.GetName(),
		UID:        hc.GetUID(),
		Controller: &trueVar,
	}
	ownerReferences = append(ownerReferences, newRef)
	workflow.SetOwnerReferences(ownerReferences)
	log.Info("Added new owner reference", "UID", newRef.UID)
	// finally, attempt to create workflow using the kube client
	err = r.Create(ctx, workflow)
	if err != nil {
		log.Error(err, "Error creating workflow")
		return "", err
	}
	generatedName := workflow.GetName()
	log.Info("Created workflow", "generatedName", generatedName)
	return generatedName, nil
}

func (r *HealthCheckReconciler) createSubmitRemedyWorkflow(ctx context.Context, log logr.Logger, hc *activemonitorv1alpha1.HealthCheck) (wfName string, err error) {
	remedyWorkflow := &unstructured.Unstructured{}
	r.parseRemedyWorkflowFromHealthcheck(log, hc, remedyWorkflow)
	remedyWorkflow.SetGroupVersionKind(wfGvk)
	remedyWorkflow.SetNamespace(hc.Spec.RemedyWorkflow.Resource.Namespace)
	remedyWorkflow.SetGenerateName(hc.Spec.RemedyWorkflow.GenerateName)
	remedyWorkflow.SetLabels(r.workflowLabels)

	// set the owner references for workflow
	ownerReferences := remedyWorkflow.GetOwnerReferences()
	trueVar := true
	newRef := metav1.OwnerReference{
		Kind:       hcKind,
		APIVersion: hcVersion,
		Name:       hc.GetName(),
		UID:        hc.GetUID(),
		Controller: &trueVar,
	}
	ownerReferences = append(ownerReferences, newRef)
	remedyWorkflow.SetOwnerReferences(ownerReferences)
	log.Info("Added new owner reference", "UID", newRef.UID)
	// finally, attempt to create workflow using the kube client
	err = r.Create(ctx, remedyWorkflow)
	if err != nil {
		log.Error(err, "Error creating remedyworkflow")
		return "", err
	}
	generatedName := remedyWorkflow.GetName()
	log.Info("Created remedyWorkflow", "generatedName", generatedName)
	return generatedName, nil
}

func (r *HealthCheckReconciler) watchWorkflowReschedule(ctx context.Context, req ctrl.Request, log logr.Logger, wfNamespace, wfName string, hc *activemonitorv1alpha1.HealthCheck) error {
	var now metav1.Time
	then := metav1.Time{Time: time.Now()}
	repeatAfterSec := hc.Spec.RepeatAfterSec
	for {
		now = metav1.Time{Time: time.Now()}
		// grab workflow object by name and check its status; update healthcheck accordingly
		// do this once per second until the workflow reaches a terminal state (success/failure)
		workflow, err := r.DynClient.Resource(wfGvr).Namespace(wfNamespace).Get(wfName, metav1.GetOptions{})
		if err != nil {
			// if the workflow wasn't found, it is most likely the case that its parent healthcheck was deleted
			// we can swallow this error and simply not reschedule
			return ignoreNotFound(err)
		}
		status, ok := workflow.UnstructuredContent()["status"].(map[string]interface{})
		if ok {
			log.Info("Workflow status", "status", status["phase"])
			if status["phase"] == succStr {
				hc.Status.Status = succStr
				hc.Status.StartedAt = &then
				hc.Status.FinishedAt = &now
				log.Info("Time:", "hc.Status.StartedAt:", hc.Status.StartedAt)
				log.Info("Time:", "hc.Status.FinishedAt:", hc.Status.FinishedAt)
				hc.Status.SuccessCount++
				hc.Status.TotalHealthCheckRuns = hc.Status.SuccessCount + hc.Status.FailedCount
				hc.Status.LastSuccessfulWorkflow = wfName
				metrics.MonitorSuccess.With(prometheus.Labels{"healthcheck_name": hc.GetName(), "workflow": healthcheck}).Inc()
				metrics.MonitorRuntime.With(prometheus.Labels{"healthcheck_name": hc.GetName(), "workflow": healthcheck}).Set(now.Time.Sub(then.Time).Seconds())
				metrics.MonitorStartedTime.With(prometheus.Labels{"healthcheck_name": hc.GetName(), "workflow": healthcheck}).Set(float64(then.Unix()))
				metrics.MonitorFinishedTime.With(prometheus.Labels{"healthcheck_name": hc.GetName(), "workflow": healthcheck}).Set(float64(hc.Status.FinishedAt.Unix()))
				break
			} else if status["phase"] == failStr {
				hc.Status.Status = failStr
				hc.Status.StartedAt = &then
				hc.Status.FinishedAt = &now
				hc.Status.LastFailedAt = &now
				hc.Status.ErrorMessage = status["message"].(string)
				hc.Status.FailedCount++
				hc.Status.TotalHealthCheckRuns = hc.Status.SuccessCount + hc.Status.FailedCount
				hc.Status.LastFailedWorkflow = wfName
				metrics.MonitorError.With(prometheus.Labels{"healthcheck_name": hc.GetName(), "workflow": healthcheck}).Inc()
				metrics.MonitorStartedTime.With(prometheus.Labels{"healthcheck_name": hc.GetName(), "workflow": healthcheck}).Set(float64(then.Unix()))
				metrics.MonitorFinishedTime.With(prometheus.Labels{"healthcheck_name": hc.GetName(), "workflow": healthcheck}).Set(float64(now.Time.Unix()))
				if !hc.Spec.RemedyWorkflow.IsEmpty() {
					err := r.processRemedyWorkflow(ctx, log, wfNamespace, hc)
					if err != nil {
						log.Error(err, "Error  executing RemedyWorkflow")
						return err
					}
				}
				break
			}
		}
		// if not breaking out due to a terminal state, sleep and check again shortly
		time.Sleep(time.Second)
	}

	// since the workflow has taken an unknown duration of time to complete, it's possible that its parent
	// healthcheck may no longer exist; ensure that it still does before attempting to update it and reschedule
	// see: https://book.kubebuilder.io/reference/using-finalizers.html
	if hc.ObjectMeta.DeletionTimestamp.IsZero() {
		// since the underlying workflow has completed, we update the healthcheck accordingly
		err := r.Update(ctx, hc)
		if err != nil {
			log.Error(err, "Error updating healthcheck resource")
		}
		// reschedule next run of workflow
		helper := r.createSubmitWorkflowHelper(ctx, log, wfNamespace, hc)
		r.RepeatTimersByName[hc.GetName()] = time.AfterFunc(time.Duration(repeatAfterSec)*time.Second, helper)
		log.Info("Rescheduled workflow for next run", "namespace", wfNamespace, "name", wfName)
	}
	return nil
}

func (r *HealthCheckReconciler) processRemedyWorkflow(ctx context.Context, log logr.Logger, wfNamespace string, hc *activemonitorv1alpha1.HealthCheck) error {

	log.Info("Creating Workflow", "namespace", wfNamespace, "generateNamePrefix", hc.Spec.RemedyWorkflow.GenerateName)
	err := r.createRBACForWorkflow(log, hc, remedy)
	if err != nil {
		log.Error(err, "Error creating RBAC Permissions for Remedy Workflow")
		return err
	}
	generatedWfName, err := r.createSubmitRemedyWorkflow(ctx, log, hc)
	if err != nil {
		log.Error(err, "Error creating or submitting remedyworkflow")
		return err
	}
	err = r.watchRemedyWorkflow(ctx, ctrl.Request{}, log, wfNamespace, generatedWfName, hc)
	if err != nil {
		log.Error(err, "Error in  watchRemedyWorkflow of remedy workflow")
		return err
	}
	err = r.deleteRBACForWorkflow(log, hc)
	if err != nil {
		log.Error(err, "Error deleting  RBAC of remedy workflow")
		return err
	}
	return nil
}

func (r *HealthCheckReconciler) watchRemedyWorkflow(ctx context.Context, req ctrl.Request, log logr.Logger, wfNamespace string, wfName string, hc *activemonitorv1alpha1.HealthCheck) error {
	var now metav1.Time
	then := metav1.Time{Time: time.Now()}
	for {
		now = metav1.Time{Time: time.Now()}
		// grab workflow object by name and check its status; update healthcheck accordingly
		// do this once per second until the workflow reaches a terminal state (success/failure)
		workflow, err := r.DynClient.Resource(wfGvr).Namespace(wfNamespace).Get(wfName, metav1.GetOptions{})
		if err != nil {
			// if the workflow wasn't found, it is most likely the case that its parent healthcheck was deleted
			// we can swallow this error and simply not reschedule
			return ignoreNotFound(err)
		}
		status, ok := workflow.UnstructuredContent()["status"].(map[string]interface{})
		if ok {
			log.Info("Workflow status", "status", status["phase"])
			if status["phase"] == succStr {
				hc.Status.RemedyStatus = succStr
				hc.Status.RemedyStartedAt = &then
				hc.Status.RemedyFinishedAt = &now
				log.Info("Time:", "hc.Status.StartedAt:", hc.Status.RemedyStartedAt)
				log.Info("Time:", "hc.Status.FinishedAt:", hc.Status.RemedyFinishedAt)
				hc.Status.RemedySuccessCount++
				hc.Status.RemedyTotalRuns = hc.Status.RemedySuccessCount + hc.Status.RemedyFailedCount
				hc.Status.LastSuccessfulWorkflow = wfName
				metrics.MonitorSuccess.With(prometheus.Labels{"healthcheck_name": hc.GetName(), "workflow": remedy}).Inc()
				metrics.MonitorRuntime.With(prometheus.Labels{"healthcheck_name": hc.GetName(), "workflow": remedy}).Set(now.Time.Sub(then.Time).Seconds())
				metrics.MonitorStartedTime.With(prometheus.Labels{"healthcheck_name": hc.GetName(), "workflow": remedy}).Set(float64(then.Unix()))
				metrics.MonitorFinishedTime.With(prometheus.Labels{"healthcheck_name": hc.GetName(), "workflow": remedy}).Set(float64(hc.Status.FinishedAt.Unix()))
				break
			} else if status["phase"] == failStr {
				hc.Status.RemedyStatus = failStr
				hc.Status.RemedyStartedAt = &then
				hc.Status.RemedyFinishedAt = &now
				hc.Status.RemedyLastFailedAt = &now
				hc.Status.RemedyErrorMessage = status["message"].(string)
				hc.Status.RemedyFailedCount++
				hc.Status.RemedyTotalRuns = hc.Status.RemedySuccessCount + hc.Status.RemedyFailedCount
				hc.Status.LastFailedWorkflow = wfName
				metrics.MonitorError.With(prometheus.Labels{"healthcheck_name": hc.GetName(), "workflow": remedy}).Inc()
				metrics.MonitorStartedTime.With(prometheus.Labels{"healthcheck_name": hc.GetName(), "workflow": remedy}).Set(float64(then.Unix()))
				metrics.MonitorFinishedTime.With(prometheus.Labels{"healthcheck_name": hc.GetName(), "workflow": remedy}).Set(float64(now.Time.Unix()))
				break
			}
		}
		// if not breaking out due to a terminal state, sleep and check again shortly
		time.Sleep(time.Second)
	}

	// since the workflow has taken an unknown duration of time to complete, it's possible that its parent
	// healthcheck may no longer exist; ensure that it still does before attempting to update it and reschedule
	// see: https://book.kubebuilder.io/reference/using-finalizers.html
	if hc.ObjectMeta.DeletionTimestamp.IsZero() {
		// since the underlying workflow has completed, we update the healthcheck accordingly
		err := r.Update(ctx, hc)
		if err != nil {
			log.Error(err, "Error updating healthcheck resource")
		}
	}

	return nil
}

func (r *HealthCheckReconciler) parseWorkflowFromHealthcheck(log logr.Logger, hc *activemonitorv1alpha1.HealthCheck, uwf *unstructured.Unstructured) error {
	var wfContent []byte
	var data map[string]interface{}
	if hc.Spec.Workflow.Resource != nil {
		reader, err := store.GetArtifactReader(&hc.Spec.Workflow.Resource.Source)
		if err != nil {
			log.Error(err, "Failed to get artifact reader")
			return err
		}
		wfContent, err = reader.Read()
		if err != nil {
			log.Error(err, "Failed to read content")
			return err
		}
	}
	// load workflow spec into data obj
	if err := yaml.Unmarshal(wfContent, &data); err != nil {
		log.Error(err, "Invalid spec file passed")
		return err
	}
	// parse workflow labels
	wflabels := data["metadata"].(map[string]interface{})["labels"]

	if r.workflowLabels == nil {
		r.workflowLabels = make(map[string]string)
	}

	//instanceId labels to workflows
	if wflabels == nil {
		r.workflowLabels[WfInstanceIdLabelKey] = WfInstanceId
	} else {
		for k, v := range wflabels.(map[string]interface{}) {
			strValue := fmt.Sprintf("%v", v)
			r.workflowLabels[k] = strValue
		}
	}
	content := uwf.UnstructuredContent()
	// make sure workflows by default get cleaned up
	if ttlSecondAfterFinished := data["spec"].(map[string]interface{})["ttlSecondsAfterFinished"]; ttlSecondAfterFinished == nil {
		data["spec"].(map[string]interface{})["ttlSecondsAfterFinished"] = defaultWorkflowTTLSec
	}
	// set service account, if specified
	if hc.Spec.Workflow.Resource.ServiceAccount != "" {
		data["spec"].(map[string]interface{})["serviceAccountName"] = hc.Spec.Workflow.Resource.ServiceAccount
		log.Info("Set ServiceAccount on Workflow", "ServiceAccount", hc.Spec.Workflow.Resource.ServiceAccount)
	}
	// and since we will reschedule workflows ourselves, we don't need k8s to try to do so for us
	var timeout int64
	timeout = int64(hc.Spec.RepeatAfterSec)
	if activeDeadlineSeconds := data["spec"].(map[string]interface{})["activeDeadlineSeconds"]; activeDeadlineSeconds == nil {
		data["spec"].(map[string]interface{})["activeDeadlineSeconds"] = &timeout
	}
	spec, ok := data["spec"]
	if !ok {
		err := errors.New("Invalid workflow, missing spec")
		log.Error(err, "Invalid workflow template spec")
		return err
	}
	content["spec"] = spec
	uwf.SetUnstructuredContent(content)
	return nil
}

func (r *HealthCheckReconciler) parseRemedyWorkflowFromHealthcheck(log logr.Logger, hc *activemonitorv1alpha1.HealthCheck, uwf *unstructured.Unstructured) error {
	var wfContent []byte
	var data map[string]interface{}
	if hc.Spec.RemedyWorkflow.Resource != nil {
		reader, err := store.GetArtifactReader(&hc.Spec.RemedyWorkflow.Resource.Source)
		if err != nil {
			log.Error(err, "Failed to get artifact reader")
			return err
		}
		wfContent, err = reader.Read()
		if err != nil {
			log.Error(err, "Failed to read content")
			return err
		}
	}
	// load workflow spec into data obj
	if err := yaml.Unmarshal(wfContent, &data); err != nil {
		log.Error(err, "Invalid spec file passed")
		return err
	}

	// parse workflow labels
	wflabels := data["metadata"].(map[string]interface{})["labels"]

	if r.workflowLabels == nil {
		r.workflowLabels = make(map[string]string)
	}

	//instanceId labels to workflows
	if wflabels == nil {
		r.workflowLabels[WfInstanceIdLabelKey] = WfInstanceId
	} else {
		for k, v := range wflabels.(map[string]interface{}) {
			strValue := fmt.Sprintf("%v", v)
			r.workflowLabels[k] = strValue
		}
	}

	content := uwf.UnstructuredContent()
	// make sure workflows by default get cleaned up
	if ttlSecondAfterFinished := data["spec"].(map[string]interface{})["ttlSecondsAfterFinished"]; ttlSecondAfterFinished == nil {
		data["spec"].(map[string]interface{})["ttlSecondsAfterFinished"] = defaultWorkflowTTLSec
	}
	// set service account, if specified
	if hc.Spec.RemedyWorkflow.Resource.ServiceAccount != "" {
		data["spec"].(map[string]interface{})["serviceAccountName"] = hc.Spec.RemedyWorkflow.Resource.ServiceAccount
		log.Info("Set ServiceAccount on Workflow", "ServiceAccount", hc.Spec.RemedyWorkflow.Resource.ServiceAccount)
	}
	// and since we will reschedule workflows ourselves, we don't need k8s to try to do so for us
	var timeout int64
	timeout = int64(hc.Spec.RepeatAfterSec)
	if activeDeadlineSeconds := data["spec"].(map[string]interface{})["activeDeadlineSeconds"]; activeDeadlineSeconds == nil {
		data["spec"].(map[string]interface{})["activeDeadlineSeconds"] = &timeout
	}
	spec, ok := data["spec"]
	if !ok {
		err := errors.New("Invalid workflow, missing spec")
		log.Error(err, "Invalid workflow template spec")
		return err
	}
	content["spec"] = spec
	uwf.SetUnstructuredContent(content)
	return nil
}

// Create ServiceAccount
func (r *HealthCheckReconciler) createServiceAccount(clientset kubernetes.Interface, name string, namespace string) (string, error) {
	sa, err := clientset.CoreV1().ServiceAccounts(namespace).Get(name, metav1.GetOptions{})
	// If a service account already exists just re-use it
	if err == nil {
		return sa.Name, nil
	}

	sa = &v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	sa, err = clientset.CoreV1().ServiceAccounts(namespace).Create(sa)
	if err != nil {
		return "", err
	}

	return sa.Name, nil
}

//Delete a service Account
func (r *HealthCheckReconciler) DeleteServiceAccount(clientset kubernetes.Interface, name string, namespace string) error {
	_, err := clientset.CoreV1().ServiceAccounts(namespace).Get(name, metav1.GetOptions{})
	// If a service account already exists just re-use it
	if err != nil {
		return err
	}

	err = clientset.CoreV1().ServiceAccounts(namespace).Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

// create a ClusterRole
func (r *HealthCheckReconciler) createClusterRole(clientset kubernetes.Interface, clusterrole string) (string, error) {
	clusrole, err := clientset.RbacV1().ClusterRoles().Get(clusterrole, metav1.GetOptions{})
	// If a Cluster Role already exists just re-use it
	if err == nil {
		return clusrole.Name, nil
	}
	clusrole, err = clientset.RbacV1().ClusterRoles().Create(&rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterrole,
		},
		Rules: []rbacv1.PolicyRule{
			{

				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	})

	if err != nil {
		return "", err
	}

	return clusrole.Name, nil
}

// create a ClusterRole
func (r *HealthCheckReconciler) createRemedyClusterRole(clientset kubernetes.Interface, clusterrole string) (string, error) {
	clusrole, err := clientset.RbacV1().ClusterRoles().Get(clusterrole, metav1.GetOptions{})
	// If a Cluster Role already exists just re-use it
	if err == nil {
		return clusrole.Name, nil
	}
	clusrole, err = clientset.RbacV1().ClusterRoles().Create(&rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterrole,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		},
	})

	if err != nil {
		return "", err
	}

	return clusrole.Name, nil
}

// Delete a ClusterRole
func (r *HealthCheckReconciler) DeleteClusterRole(clientset kubernetes.Interface, clusterrole string) error {
	_, err := clientset.RbacV1().ClusterRoles().Get(clusterrole, metav1.GetOptions{})
	// If a Cluster Role already exists just re-use it
	if err != nil {
		return err
	}

	err = clientset.RbacV1().ClusterRoles().Delete(clusterrole, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

// Create NamespaceRole
func (r *HealthCheckReconciler) createNameSpaceRole(clientset kubernetes.Interface, nsrole string, namespace string) (string, error) {
	nsrole1, err := clientset.RbacV1().Roles(namespace).Get(nsrole, metav1.GetOptions{})
	// If a Namespace Role already exists just re-use it
	if err == nil {
		return nsrole1.Name, nil
	}
	nsrole1, err = clientset.RbacV1().Roles(namespace).Create(&rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsrole,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	})
	if err != nil {
		return "", err
	}

	return nsrole1.Name, nil
}

// Create NamespaceRole
func (r *HealthCheckReconciler) createRemedyNameSpaceRole(clientset kubernetes.Interface, nsrole string, namespace string) (string, error) {
	nsrole1, err := clientset.RbacV1().Roles(namespace).Get(nsrole, metav1.GetOptions{})
	// If a Namespace Role already exists just re-use it
	if err == nil {
		return nsrole1.Name, nil
	}
	nsrole1, err = clientset.RbacV1().Roles(namespace).Create(&rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsrole,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		},
	})
	if err != nil {
		return "", err
	}

	return nsrole1.Name, nil
}

// Delete NamespaceRole
func (r *HealthCheckReconciler) DeleteNameSpaceRole(clientset kubernetes.Interface, nsrole string, namespace string) error {
	// Check if a Namespace Role already exists
	_, err := clientset.RbacV1().Roles(namespace).Get(nsrole, metav1.GetOptions{})
	if err != nil {
		return err
	}

	err = clientset.RbacV1().Roles(namespace).Delete(nsrole, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

// Create a NamespaceRoleBinding
func (r *HealthCheckReconciler) createNameSpaceRoleBinding(clientset kubernetes.Interface, rolebinding string, nsrole string, serviceaccount string, namespace string) (string, error) {
	nsrb, err := clientset.RbacV1().RoleBindings(namespace).Get(rolebinding, metav1.GetOptions{})
	// If a Namespace RoleBinding already exists just re-use it
	if err == nil {
		return nsrb.Name, nil
	}
	nsrb, err = clientset.RbacV1().RoleBindings(namespace).Create(&rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rolebinding,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: serviceaccount,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     nsrole,
			APIGroup: "rbac.authorization.k8s.io",
		},
	})
	if err != nil {
		return "", err
	}

	return nsrb.Name, nil
}

// Delete NamespaceRoleBinding
func (r *HealthCheckReconciler) DeleteNameSpaceRoleBinding(clientset kubernetes.Interface, rolebinding string, nsrole string, serviceaccount string, namespace string) error {
	// Check if a Namespace RoleBinding exists
	_, err := clientset.RbacV1().RoleBindings(namespace).Get(rolebinding, metav1.GetOptions{})
	if err != nil {
		return err
	}
	err = clientset.RbacV1().RoleBindings(namespace).Delete(rolebinding, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

// Create a ClusterRoleBinding
func (r *HealthCheckReconciler) createClusterRoleBinding(clientset kubernetes.Interface, clusterrolebinding string, clusterrole string, serviceaccount string, namespace string) (string, error) {
	crb, err := clientset.RbacV1().ClusterRoleBindings().Get(clusterrolebinding, metav1.GetOptions{})
	// If a Cluster RoleBinding already exists just re-use it
	if err == nil {
		return crb.Name, nil
	}
	crb, err = clientset.RbacV1().ClusterRoleBindings().Create(&rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterrolebinding,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceaccount,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     clusterrole,
			APIGroup: "rbac.authorization.k8s.io",
		},
	})
	if err != nil {
		return "", err
	}

	return crb.Name, nil

}

// Delete ClusterRoleBinding
func (r *HealthCheckReconciler) DeleteClusterRoleBinding(clientset kubernetes.Interface, clusterrolebinding string, clusterrole string, serviceaccount string, namespace string) error {
	// Check if a Cluster RoleBinding exists
	_, err := clientset.RbacV1().ClusterRoleBindings().Get(clusterrolebinding, metav1.GetOptions{})
	if err != nil {
		return err
	}

	err = clientset.RbacV1().ClusterRoleBindings().Delete(clusterrolebinding, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil

}
