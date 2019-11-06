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
	"k8s.io/api/core/v1"
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
	hcKind                = "HealthCheck"
	hcVersion             = "v1alpha1"
	wfGroup               = "argoproj.io"
	wfVersion             = "v1alpha1"
	wfKind                = "Workflow"
	wfResource            = "workflows"
	succStr               = "Succeeded"
	failStr               = "Failed"
	defaultWorkflowTTLSec = 1800
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

	// initialize timers map if not already done
	if r.RepeatTimersByName == nil {
		r.RepeatTimersByName = make(map[string]*time.Timer)
	}

	var healthCheck activemonitorv1alpha1.HealthCheck
	if err := r.Get(ctx, req.NamespacedName, &healthCheck); err != nil {
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
	hcSpec := healthCheck.Spec
	if hcSpec.Workflow.Resource != nil {
		wfNamePrefix := hcSpec.Workflow.GenerateName
		wfNamespace := hcSpec.Workflow.Resource.Namespace
		repeatAfterSec := hcSpec.RepeatAfterSec
		level := hcSpec.Level
		sa := hcSpec.Workflow.Resource.ServiceAccount
		now := metav1.Time{Time: time.Now()}
		var finishedAtTime int64
		if healthCheck.Status.FinishedAt != nil {
			finishedAtTime = healthCheck.Status.FinishedAt.Time.Unix()
		}
		// workflows can be paused by setting repeatAfterSec to <= 0.
		if repeatAfterSec <= 0 {
			log.Info("Workflow will be skipped due to repeatAfterSec value", "repeatAfterSec", repeatAfterSec)
			healthCheck.Status.Status = "Stopped"
			healthCheck.Status.ErrorMessage = fmt.Sprintf("workflow execution is stopped due to spec.repeatAfterSec set to %d", repeatAfterSec)
			healthCheck.Status.FinishedAt = &now
			return ctrl.Result{}, nil
		} else if int(time.Now().Unix()-finishedAtTime) < repeatAfterSec {
			log.Info("Workflow already executed", "finishedAtTime", finishedAtTime)
			return ctrl.Result{}, nil
		}
		if level == "cluster" {
			amclusterrole := sa + "-cluster-role"
			amclusterrolebinding := sa + "-cluster-role-binding"
			servacc, err := r.CreateServiceAccount(r.kubeclient, sa, wfNamespace)
			if err != nil {
				log.Error(err, "Error creating ServiceAccount for the workflow")
			}
			log.Info("Successfully Created", "ServiceAccount", servacc)

			clusrole, err := r.createClusterRole(r.kubeclient, amclusterrole)
			if err != nil {
				log.Error(err, "Error creating ClusterRole for the workflow")
			}
			log.Info("Successfully Created", "ClusterRole", clusrole)

			crb, err := r.CreateClusterRoleBinding(r.kubeclient, amclusterrolebinding, clusrole, sa, wfNamespace)
			if err != nil {
				log.Error(err, "Error creating ClusterRoleBinding for the workflow")
			}
			log.Info("Successfully Created", "ClusterRoleBinding", crb)

		} else if level == "namespace" {
			amnsrole := sa + "-ns-role"
			amnsrolebinding := sa + "-ns-role-binding"
			servacc, err := r.CreateServiceAccount(r.kubeclient, sa, wfNamespace)
			if err != nil {
				log.Error(err, "Error creating ServiceAccount for the workflow")
			}
			log.Info("Successfully Created", "ServiceAccount", servacc)

			nsrole, err := r.CreateNameSpaceRole(r.kubeclient, amnsrole, wfNamespace)
			if err != nil {
				log.Error(err, "Error creating NamespaceRole for the workflow")
			}
			log.Info("Successfully Created", "NamespaceRole", nsrole)

			nsrb, err := r.CreateNameSpaceRoleBinding(r.kubeclient, amnsrolebinding, nsrole, sa, wfNamespace)
			if err != nil {
				log.Error(err, "Error creating NamespaceRoleBinding for the workflow")
			}
			log.Info("Successfully Created", "NamespaceRoleBinding", nsrb)

		} else {
			return ctrl.Result{}, nil
		}

		log.Info("Creating Workflow", "namespace", wfNamespace, "generateNamePrefix", wfNamePrefix)
		generatedWfName, err := r.createSubmitWorkflow(ctx, log, &healthCheck)
		if err != nil {
			log.Error(err, "Error creating or submitting workflow")
		}
		r.watchWorkflowReschedule(ctx, log, wfNamespace, generatedWfName, &healthCheck)
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
		err = r.watchWorkflowReschedule(ctx, log, wfNamespace, wfName, hc)
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

func (r *HealthCheckReconciler) watchWorkflowReschedule(ctx context.Context, log logr.Logger, wfNamespace, wfName string, hc *activemonitorv1alpha1.HealthCheck) error {
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
				hc.Status.FinishedAt = &now
				hc.Status.SuccessCount++
				hc.Status.LastSuccessfulWorkflow = wfName
				metrics.MonitorSuccess.With(prometheus.Labels{"healthcheck_name": hc.GetName()}).Inc()
				metrics.MonitorRuntime.With(prometheus.Labels{"healthcheck_name": hc.GetName()}).Set(now.Time.Sub(then.Time).Seconds())
				break
			} else if status["phase"] == failStr {
				hc.Status.Status = failStr
				hc.Status.FinishedAt = &now
				hc.Status.LastFailedAt = &now
				hc.Status.ErrorMessage = status["message"].(string)
				hc.Status.FailedCount++
				hc.Status.LastFailedWorkflow = wfName
				metrics.MonitorError.With(prometheus.Labels{"healthcheck_name": hc.GetName()}).Inc()
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

// Create ServiceAccount
func (r *HealthCheckReconciler) CreateServiceAccount(clientset kubernetes.Interface, name string, namespace string) (string, error) {
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

// create a ClusterRole account
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

// Create NamespaceRole
func (r *HealthCheckReconciler) CreateNameSpaceRole(clientset kubernetes.Interface, nsrole string, namespace string) (string, error) {
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

// Create a NamespaceRoleBinding
func (r *HealthCheckReconciler) CreateNameSpaceRoleBinding(clientset kubernetes.Interface, rolebinding string, nsrole string, serviceaccount string, namespace string) (string, error) {
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

// Create a ClusterRoleBinding
func (r *HealthCheckReconciler) CreateClusterRoleBinding(clientset kubernetes.Interface, clusterrolebinding string, clusterrole string, serviceaccount string, namespace string) (string, error) {
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
