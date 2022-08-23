// Copyright 2019 FairwindsOps Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validator

import (
	"strings"

	conf "github.com/choerodon/choerodon-cluster-agent/pkg/polaris/config"
	"github.com/choerodon/choerodon-cluster-agent/pkg/polaris/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/polaris/validator/controllers"
	controller "github.com/choerodon/choerodon-cluster-agent/pkg/polaris/validator/controllers"
)

const exemptionAnnotationKey = "polaris.fairwinds.com/exempt"

// ValidateController validates a single controller, returns a ControllerResult.
func ValidateController(conf *conf.Configuration, controller controller.Interface) (ControllerResult, error) {
	podResult, err := ValidatePod(conf, controller)
	if err != nil {
		return ControllerResult{}, err
	}
	result := ControllerResult{
		Kind:      controller.GetKind().String(),
		Name:      controller.GetName(),
		Namespace: controller.GetObjectMeta().Namespace,
		Results:   ResultSet{},
		PodResult: podResult,
	}
	return result, nil
}

// ValidateControllers validates that each deployment conforms to the Polaris config,
// builds a list of ResourceResults organized by namespace.
func ValidateControllers(config *conf.Configuration, kubeResources *kube.ResourceProvider) ([]ControllerResult, error) {
	var controllersToAudit []controller.Interface
	for _, supportedControllers := range config.ControllersToScan {
		loadedControllers, _ := controllers.LoadControllersByKind(supportedControllers, kubeResources)
		controllersToAudit = append(controllersToAudit, loadedControllers...)
	}

	results := []ControllerResult{}
	for _, controller := range controllersToAudit {
		if !config.DisallowExemptions && hasExemptionAnnotation(controller) {
			continue
		}
		result, err := ValidateController(config, controller)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func hasExemptionAnnotation(ctrl controller.Interface) bool {
	annot := ctrl.GetObjectMeta().Annotations
	val := annot[exemptionAnnotationKey]
	return strings.ToLower(val) == "true"
}
