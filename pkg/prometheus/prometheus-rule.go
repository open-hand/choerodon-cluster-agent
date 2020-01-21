package prometheus

type UnstructuredObject struct {
	Object PrometheusRule
}

type PrometheusRule struct {
	ApiVersion string   `yaml:"apiVersion" json:"apiVersion"`
	Kind       string   `yaml:"kind" json:"kind"`
	Metadata   Metadata `yaml:"metadata" json:"metadata"`
	Spec       Spec     `yaml:"spec" json:"spec"`
}

type Metadata struct {
	Name      string            `yaml:"name" json:"name"`
	Namespace string            `yaml:"namespace" json:"namespace"`
	Labels    map[string]string `yaml:"labels" json:"labels"`
}

type Spec struct {
	Groups []Group `yaml:"groups" json:"groups"`
}

type Group struct {
	Name  string `yaml:"name" json:"name"`
	Rules []Rule `yaml:"rules" json:"rules"`
}

type Rule struct {
	Alert       string            `yaml:"alert" json:"alert"`
	Annotations map[string]string `yaml:"annotations" json:"annotations"`
	Expr        string            `yaml:"expr" json:"expr"`
	For         string            `yaml:"for" json:"for"`
	Labels      map[string]string `yaml:"labels" json:"labels"`
}
