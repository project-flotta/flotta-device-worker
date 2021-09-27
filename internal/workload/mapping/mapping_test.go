package mapping_test

import (
	"github.com/jakub-dzon/k4e-device-worker/internal/workload/mapping"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"os"
)

var _ = Describe("Mapping", func() {

	var configDir string
	var repo *mapping.MappingRepository

	BeforeEach(func() {
		dir, err := ioutil.TempDir(os.TempDir(), "mapping")
		Expect(err).ToNot(HaveOccurred())
		configDir = dir

		repo, err = mapping.NewMappingRepository(dir)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should be created empty", func() {
		// then
		Expect(repo).ToNot(BeNil())
		Expect(repo.Size()).To(BeZero())
	})

	It("should return empty string for non-existing name", func() {
		// when
		id := repo.GetId("not-here")

		// then
		Expect(id).To(BeEmpty())
	})

	It("should return empty string for non-existing ID", func() {
		// when
		name := repo.GetName("not here")

		// then
		Expect(name).To(BeEmpty())
	})

	It("should store and return values", func() {
		// when
		err := repo.Add("test", "test-id")

		// then
		Expect(err).ToNot(HaveOccurred())

		Expect(repo.GetId("test")).To(BeEquivalentTo("test-id"))
		Expect(repo.GetName("test-id")).To(BeEquivalentTo("test"))
	})

	It("should remove mapping", func() {
		// given
		err := repo.Add("test", "test-id")
		Expect(err).ToNot(HaveOccurred())

		// when
		err = repo.Remove("test")

		// then
		Expect(err).ToNot(HaveOccurred())
		Expect(repo.GetId("test")).To(BeEmpty())
		Expect(repo.GetName("test-id")).To(BeEmpty())
	})

	It("should persist mappings", func() {
		// given
		err := repo.Add("one-name", "one-id")
		Expect(err).ToNot(HaveOccurred())

		err = repo.Add("two-name", "two-id")
		Expect(err).ToNot(HaveOccurred())

		// when
		repo2, err := mapping.NewMappingRepository(configDir)

		// then
		Expect(err).ToNot(HaveOccurred())

		Expect(repo2.GetId("one-name")).To(BeEquivalentTo("one-id"))
		Expect(repo2.GetId("two-name")).To(BeEquivalentTo("two-id"))

		Expect(repo2.GetName("one-id")).To(BeEquivalentTo("one-name"))
		Expect(repo2.GetName("two-id")).To(BeEquivalentTo("two-name"))
	})

})
