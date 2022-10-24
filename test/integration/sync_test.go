package e2e_test

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stolostron/cluster-imageset-controller/test/integration/util"
)

var _ = ginkgo.Describe("Sync", func() {
	ginkgo.Context("check cluster imageset is imported", func() {
		ginkgo.It("cluster imageset exist", func() {
			ginkgo.By("Check the cluster imageset is imported")
			gomega.Eventually(func() bool {
				imagesetList, err := util.GetClusterImageSets(dynamicClient)
				if err != nil {
					return false
				}

				if len(imagesetList.Items) == 0 {
					return false
				}

				return true
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
		})
	})
})
