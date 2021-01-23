package controllers

import (
	"context"
	"github.com/Dana-Team/SNS/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const (
	timeOut      = time.Second * 50
	intervalTime = time.Millisecond * 250
)

var (
	parentNamespaceName = "sns-system-tests"
	subnamespaceName    = "child"
	childNamespaceName  = parentNamespaceName + "-" + subnamespaceName
)

var (
	childNamespace = v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: childNamespaceName,
		},
	}

	testsNamespace = v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: parentNamespaceName,
			Annotations: map[string]string{
				v1alpha1.Role: v1alpha1.Root,
			},
		},
	}

	subnamespace = v1alpha1.Subnamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      subnamespaceName,
			Namespace: parentNamespaceName,
		},
	}
)

var _ = Describe("Subnamespace controller tests", func() {
	ctx := context.Background()
	Context("With creating a Parent Namespace", func() {
		It("Should create a Parent Namespace", func() {
			Expect(k8sClient.Create(ctx, &testsNamespace)).Should(Succeed())
		})
	})
	Context("With creating a Subnamespace", func() {
		It("Should create a Subnamespace and a Namespace owned by this Subnamespace", func() {
			Expect(k8sClient.Create(ctx, &subnamespace)).Should(Succeed())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "", Name: childNamespaceName}, &childNamespace); err != nil {
					return false
				}
				return true
			}, timeOut, intervalTime).Should(BeTrue())
		})
	})
	Context("With cleaning up the test child Namespace", func() {
		It("Should delete the owner Subnamespace and its Namespace", func() {
			Expect(k8sClient.Delete(ctx, &childNamespace))
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "", Name: childNamespaceName}, &childNamespace); err != nil {
					// The error should be NotFound.
					if apierrors.IsNotFound(err) {
						return true
					}
				}
				return false
			}, timeOut, intervalTime).Should(BeTrue())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: parentNamespaceName, Name: subnamespaceName}, &subnamespace); err != nil {
					// The error should be NotFound.
					if apierrors.IsNotFound(err) {
						return true
					}
				}
				return false
			}, timeOut, intervalTime).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, &testsNamespace))
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "", Name: parentNamespaceName}, &testsNamespace); err != nil {
					// The error should be NotFound.
					if apierrors.IsNotFound(err) {
						return true
					}
				}
				return false
			}, timeOut, intervalTime).Should(BeTrue())
		})
	})
})
