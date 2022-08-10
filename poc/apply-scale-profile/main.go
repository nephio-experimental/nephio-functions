package main

import (
	"fmt"
	"os"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	corev1 "k8s.io/api/core/v1"
	yaml "sigs.k8s.io/kustomize/kyaml/yaml"

	infrav1alpha1 "github.com/nephio-project/nephio-controller-poc/apis/infra/v1alpha1"
)

func Run(rl *fn.ResourceList) (bool, error) {
	var fc applyScaleProfileConfig

	if err := rl.FunctionConfig.As(&fc); err != nil {
		return false, fmt.Errorf("could not parse function config: %s", err)
	}

	if fc.ProfileName == "" {
		return false, fmt.Errorf("profile is required in function config")
	}

	// find the scale profile
	var sp *fn.KubeObject
	for _, obj := range rl.Items {
		if obj.IsGVK(infrav1alpha1.GroupVersion.Group, infrav1alpha1.GroupVersion.Version, "ClusterScaleProfile") && obj.GetName() == fc.ProfileName {
			sp = obj
			continue
		}
	}

	if sp == nil {
		return false, fmt.Errorf("could not find %s/%s ClusterScaleProfile named %q",
			infrav1alpha1.GroupVersion.Group, infrav1alpha1.GroupVersion.Version, fc.ProfileName)
	}

	// loop through each configMapScalePolicy, and apply the profile to each
	for _, p := range fc.ConfigMaps {
		if err := p.applyScaleProfile(rl, sp); err != nil {
			return false, err
		}
	}

	// loop through each deploymentScalePolicy, and apply the profile to each
	for _, p := range fc.Deployments {
		if err := p.applyScaleProfile(rl, sp); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (p configMapScalePolicy) applyScaleProfile(rl *fn.ResourceList, sp *fn.KubeObject) error {
	// find the config map
	var cm *fn.KubeObject
	for _, obj := range rl.Items {
		if obj.IsGVK("", "v1", "ConfigMap") && obj.GetName() == p.Name {
			cm = obj
			break
		}
	}

	if cm == nil {
		return fmt.Errorf("ConfigMap %q not found in resource list", p.Name)
	}

	siteDensity := sp.NestedStringOrDie("spec", "siteDensity")
	scaledKey := p.Key + "-" + siteDensity
	data := cm.NestedStringMapOrDie("data")
	scaledValue, ok := data[scaledKey]
	if !ok {
		return fmt.Errorf("key %q not found in ConfigMap %q", scaledKey, p.Name)
	}
	data[p.Key] = scaledValue
	cm.SetNestedStringMapOrDie(data, "data")

	return nil
}

func (p deploymentScalePolicy) applyScaleProfile(rl *fn.ResourceList, sp *fn.KubeObject) error {
	return nil
}

type applyScaleProfileConfig struct {
	yaml.ResourceIdentifier `json:",inline" yaml:",inline"`
	ProfileName             string                  `json:"profile,omitempty" yaml:"profile,omitempty"`
	ConfigMaps              []configMapScalePolicy  `json:"configMaps,omitempty" yaml:"configMaps,omitempty"`
	Deployments             []deploymentScalePolicy `json:"deployments,omitempty" yaml:"deployments,omitempty"`
}

type configMapScalePolicy struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	Key  string `json:"key,omitempty" yaml:"key,omitempty"`
}

type deploymentScalePolicy struct {
	Name          string                   `json:"name,omitempty" yaml:"name,omitempty"`
	LowDensity    deploymentPodScalePolicy `json:"lowDensity,omitempty" yaml:"lowDensity,omitempty"`
	MediumDensity deploymentPodScalePolicy `json:"mediumDensity,omitempty" yaml:"mediumDensity,omitempty"`
	HighDensity   deploymentPodScalePolicy `json:"highDensity,omitempty" yaml:"highDensity,omitempty"`
}

type deploymentPodScalePolicy struct {
	Replicas   int                              `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Containers []deploymentContainerScalePolicy `json:"containers,omitempty" yaml:"containers,omitempty"`
}

type deploymentContainerScalePolicy struct {
	Name      string                      `json:"name,omitempty" yaml:"name,omitempty"`
	Resources corev1.ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
}

func main() {
	if err := fn.AsMain(fn.ResourceListProcessorFunc(Run)); err != nil {
		os.Exit(1)
	}
}
