package configmaps

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func CreateConfigMap(name string, content map[string]string) ([]byte, error) {
	cm := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind: "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data:       content,
	}
	return yaml.Marshal(cm)
}
