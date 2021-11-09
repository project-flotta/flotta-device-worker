package configmaps_test

import (
	"github.com/jakub-dzon/k4e-device-worker/internal/configmaps"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

var _ = Describe("ConfigMaps", func() {

	mapWithValues := map[string]string{"ONE": "one", "TWO": "two"}

	table.DescribeTable("should create ConfigMap YAML", func(content map[string]string, expectedContent map[string]string) {
		// given
		name := "some-cm"

		// when
		cmYaml, err := configmaps.CreateConfigMap(name, content)

		// then
		Expect(err).ToNot(HaveOccurred())
		Expect(cmYaml).ToNot(BeNil())

		cm := v1.ConfigMap{}
		err = yaml.Unmarshal(cmYaml, &cm)
		Expect(err).ToNot(HaveOccurred())
		Expect(cm.Name).To(BeEquivalentTo(name))
		Expect(cm.Kind).To(BeEquivalentTo("ConfigMap"))
		Expect(cm.Data).To(BeEquivalentTo(expectedContent))
	},
		table.Entry("Map with values", mapWithValues, mapWithValues),
		table.Entry("Nil map", nil, nil),
		table.Entry("Empty map", map[string]string{}, nil),
	)
})
