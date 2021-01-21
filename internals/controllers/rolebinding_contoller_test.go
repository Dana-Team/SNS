package controllers

import (
	"context"
	"fmt"
	"github.com/Dana-Team/SNS/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1api "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var (
	rootnsname = "rbtests14"
	RbName     = "rbtest"
	SubRbNs    = rootnsname + "-sub"
)

// define RoleBindnigs and Namaespaces
var (
	FirstRb = rbacv1.RoleBinding{
		ObjectMeta: v1api.ObjectMeta{
			Name:      RbName,
			Namespace: rootnsname,
		},
		Subjects: []rbacv1.Subject{rbacv1.Subject{
			Kind:      "User",
			APIGroup:  "rbac.authorization.k8s.io",
			Name:      RbName,
			Namespace: rootnsname,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     RbName,
		},
	}

	SubRb = rbacv1.RoleBinding{
		ObjectMeta: v1api.ObjectMeta{
			Name:      RbName,
			Namespace: SubRbNs,
		},
		Subjects: []rbacv1.Subject{rbacv1.Subject{
			Kind:      "User",
			APIGroup:  "rbac.authorization.k8s.io",
			Name:      RbName,
			Namespace: SubRbNs,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     RbName,
		},
	}

	ChecksRb = rbacv1.RoleBinding{}

	rootNs = v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: rootnsname,
			Annotations: map[string]string{
				v1alpha1.Role: v1alpha1.Root,
			},
		},
	}

	SubNs = v1alpha1.Subnamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sub",
			Namespace: rootnsname,
		},
	}
)

const (
	timeout  = time.Second * 60
	interval = time.Millisecond * 250
)

func containsFinalizer(finalizers []string) bool {
	for _, finalizer := range finalizers {
		if finalizer == v1alpha1.RbFinalizer {
			return true
		}
	}
	return false
}

var _ = Describe("RoleBinding controller", func() {
	ctx := context.Background()

	fmt.Print("\n Crating new root namespace: ", rootnsname, "\n")

	Context("INIT", func() {
		It("INIT", func() {
			By("INIT")
			Expect(k8sClient.Create(ctx, &rootNs)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &SubNs)).Should(Succeed())
		})
	})

	Context("When root rb is created", func() {
		It("Should create a rolebinding in root and sub namespace", func() {
			By("Creating a rolebinding  in root namespace")
			//Creating the root rolebinding - we expect it will succeed
			Expect(k8sClient.Create(ctx, &FirstRb)).Should(Succeed())
			//Eventually
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: SubRb.Subjects[0].Namespace, Name: SubRb.Subjects[0].Name}, &ChecksRb); err != nil {
					fmt.Print("Get Fail")

					return false
				}

				if !containsFinalizer(ChecksRb.Finalizers) {
					fmt.Print("Finalizer Fail")
					return false
				}
				fmt.Print("ALl Passed ")
				return true

			}, timeout, interval).Should(BeTrue())

		})
	})

	Context("When rb is deleted", func() {
		It("should delete the rb from sub namespace", func() {
			By("Deleting rb in sub namespace")
			Expect(k8sClient.Delete(context.Background(), &FirstRb)).Should(Succeed())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: SubRbNs, Name: SubRb.Name}, &ChecksRb); err == nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})
	fmt.Print("\n Deleting new root namespace", rootnsname, "\n")

	Context("Deleting the root namesapce", func() {
		It("Deleting the root namesapce", func() {
			By("Deleting the root namesapce")
			Expect(k8sClient.Delete(ctx, &rootNs)).Should(Succeed())

		})
	})
})
