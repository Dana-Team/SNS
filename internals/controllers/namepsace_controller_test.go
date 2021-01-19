package controllers

import (
	"context"
	"github.com/Dana-Team/SNS/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

var (
	rootNsName = "rootns"

	childSnsName = "chidsns"
	childNsName  = rootNsName + "-" + childSnsName

	grandChildSnsName = "grandchildsns"
	grandChildNsName  = childNsName + "-" + grandChildSnsName
)

var (
	rootNs = v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: rootNsName,
			Annotations: map[string]string{
				v1alpha1.Role: v1alpha1.Root,
			},
		},
	}

	chidSns = v1alpha1.Subnamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      childSnsName,
			Namespace: rootNsName,
		},
	}

	grandChildSns = v1alpha1.Subnamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      grandChildSnsName,
			Namespace: childNsName,
		},
	}
)

var (
	childNs      = v1.Namespace{}
	grandChildNs = v1.Namespace{}
)

const (
	timeout  = time.Second * 30
	interval = time.Millisecond * 250
)

var _ = Describe("Namespace controller", func() {
	ctx := context.Background()
	Context("When root Namespace is created", func() {
		It("Should create a root namespace", func() {
			By("Creating a root namespace")
			Expect(k8sClient.Create(ctx, &rootNs)).Should(Succeed())
		})
	})
	Context("When SNS is created in the namespace", func() {
		It("Should create a child namespace", func() {
			By("Creating a SNS in root namespace")
			Expect(k8sClient.Create(ctx, &chidSns)).Should(Succeed())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name: childNsName,
				}, &childNs); err != nil {
					return false
				}
				if childNs.Annotations[v1alpha1.Role] == v1alpha1.Leaf && containsFinalizer(childNs.Finalizers) {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

		})
		It("Should create a grandchild namespace", func() {
			By("Creating a SNS in child namespace")
			Expect(k8sClient.Create(ctx, &grandChildSns)).Should(Succeed())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name: grandChildNsName,
				}, &grandChildNs); err != nil {
					return false
				}
				if grandChildNs.Annotations[v1alpha1.Role] == v1alpha1.Leaf && containsFinalizer(grandChildNs.Finalizers) {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name: childNsName,
				}, &childNs); err != nil {
					return false
				}
				if childNs.Annotations[v1alpha1.Role] == v1alpha1.NoRole {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

		})
	})
	Context("When Namespace is deleted", func() {
		It("should delete sns from child namespace", func() {
			By("Deleting grandchild namespace")
			Expect(k8sClient.Delete(context.Background(), &grandChildNs)).Should(Succeed())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: childNsName,
					Name:      grandChildSnsName,
				}, &v1alpha1.Subnamespace{}); err != nil {
					return apierrors.IsNotFound(err)
				}
				return false
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name: childNsName,
				}, &childNs); err != nil {
					return false
				}
				if childNs.Annotations[v1alpha1.Role] == v1alpha1.Leaf {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

		})
		It("should delete sns from root namespace", func() {
			By("Deleting child namespace")
			Expect(k8sClient.Delete(context.Background(), &childNs)).Should(Succeed())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: rootNsName,
					Name:      childSnsName,
				}, &v1alpha1.Subnamespace{}); err != nil {
					return apierrors.IsNotFound(err)
				}
				return false
			}, timeout, interval).Should(BeTrue())

		})
		It("should delete root namespace", func() {
			By("Deleting root namespace")
			Expect(k8sClient.Delete(context.Background(), &rootNs)).Should(Succeed())
		})
	})
})

func containsFinalizer(finalizers []string) bool {
	for _, finalizer := range finalizers {
		if finalizer == v1alpha1.NsFinalizer {
			return true
		}
	}
	return false
}
