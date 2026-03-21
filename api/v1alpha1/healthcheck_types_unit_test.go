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

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemedyWorkflow_IsEmpty_ZeroValue(t *testing.T) {
	assert.True(t, RemedyWorkflow{}.IsEmpty())
}

func TestRemedyWorkflow_IsEmpty_WithGenerateName(t *testing.T) {
	assert.False(t, RemedyWorkflow{GenerateName: "remedy-"}.IsEmpty())
}

func TestRemedyWorkflow_IsEmpty_WithResource(t *testing.T) {
	assert.False(t, RemedyWorkflow{Resource: &ResourceObject{Namespace: "health"}}.IsEmpty())
}

func TestRemedyWorkflow_IsEmpty_WithTimeout(t *testing.T) {
	assert.False(t, RemedyWorkflow{Timeout: 30}.IsEmpty())
}
