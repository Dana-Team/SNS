package controllers

import (
	"context"
	"fmt"
	"github.com/Dana-Team/SNS/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
)

var (
	childSns = composeSns(&rootNs)
	childNs  = getChildNs(&childSns)

	grandChildSns = composeSns(&childNs)
	grandChildNs  = getChildNs(&grandChildSns)
)

var _ = Describe("Namespace controller", func() {
	fmt.Println(childSns.Name, childSns.Namespace)
	fmt.Println(childNs.Name)
	fmt.Println(grandChildSns.Name, grandChildSns.Namespace)
	fmt.Println(grandChildNs.Name)
	ctx := context.Background()

	Context("Namespace Init", func() {
		It("Should create a child namespace", func() {
			By("Creating a SNS in parent namespace")

			Expect(k8sClient.Create(ctx, &childSns)).Should(Succeed())
			isCreated(ctx, k8sClient, &childNs)

		})

		It("Should copy parent role bindings", func() {

			childRb := getRbForNs(&rootRb, &childNs)
			isCreated(ctx, k8sClient, &childRb)

		})
	})

	Context("Namespace Update", func() {
		It("Should update child namespace role to None", func() {
			By("Creating a SNS in child namespace")

			Expect(k8sClient.Create(ctx, &grandChildSns)).Should(Succeed())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name: childNs.Name,
				}, &childNs); err != nil {
					return false
				}
				if childNs.Annotations[v1alpha1.Role] == v1alpha1.NoRole {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

		})
		It("Should update child namespace role to Leaf", func() {
			By("Deleting a SNS in child namespace")

			Expect(k8sClient.Delete(ctx, &grandChildNs)).Should(Succeed())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name: childNs.Name,
				}, &childNs); err != nil {
					return false
				}
				if childNs.Annotations[v1alpha1.Role] == v1alpha1.Leaf {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

		})

	})
	Context("Namespace Delete", func() {
		It("Should clean up after the namespace", func() {
			By("Deleting the namespace")

			Expect(k8sClient.Delete(ctx, &childNs)).Should(Succeed())
			isDeleted(ctx, k8sClient, &childSns)

		})

		It("Should delete namespace crq", func() {

			childCrq := getCrqForNs(&childNs)
			isDeleted(ctx, k8sClient, &childCrq)

		})
	})
})
