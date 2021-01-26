package controllers

import (
	"context"
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"

	//. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	danav1alpha1 "github.com/Dana-Team/SNS/api/v1alpha1"
	qoutav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/rand"
)

const (
	timeout  = time.Second * 30
	interval = time.Millisecond * 250
)

var (
	defaultQuantity       = int64(10)
	defaultQuantityFormat = resource.DecimalSI
	defaultResources      = []corev1.ResourceName{"cpu", "memory"}
)

func getRandomName(prefix string) string {
	rand.Seed(time.Now().UnixNano())
	suffix := make([]byte, 5)
	rand.Read(suffix)
	return fmt.Sprintf("%s%x", prefix, suffix)
}

func composeRootNs() corev1.Namespace {
	return corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        getRandomName("ns"),
			Labels:      map[string]string{danav1alpha1.Hns: "True"},
			Annotations: map[string]string{danav1alpha1.Role: danav1alpha1.Root, danav1alpha1.Depth: "0"},
		},
	}
}

func composeRootRb(namespace *corev1.Namespace) rbacv1.RoleBinding {
	return rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getRandomName("rb"),
			Namespace: namespace.Name,
			Labels:    map[string]string{danav1alpha1.Hns: "True"},
		},
		Subjects: []rbacv1.Subject{{
			Kind: "User",
			Name: getRandomName("subject"),
		}},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "admin",
		},
	}
}

func composeRootCrq(namespace *corev1.Namespace) qoutav1.ClusterResourceQuota {
	return qoutav1.ClusterResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:   getRandomName("crq"),
			Labels: map[string]string{danav1alpha1.Hns: "True"},
		},
		Spec: qoutav1.ClusterResourceQuotaSpec{
			Selector: qoutav1.ClusterResourceQuotaSelector{
				AnnotationSelector: map[string]string{fmt.Sprintf("%s-%s", danav1alpha1.CrqSelector, "0"): namespace.Name},
			},
			Quota: corev1.ResourceQuotaSpec{
				Hard: composeQuota(defaultQuantity),
			},
		},
	}
}

func composeQuota(quantity int64) map[corev1.ResourceName]resource.Quantity {
	quota := make(map[corev1.ResourceName]resource.Quantity)
	for _, r := range defaultResources {
		quota[r] = *resource.NewQuantity(quantity, defaultQuantityFormat)
	}
	return quota
}

func composeSns(namespace *corev1.Namespace) danav1alpha1.Subnamespace {
	return danav1alpha1.Subnamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getRandomName("sns"),
			Namespace: namespace.Name,
		},
		Spec: danav1alpha1.SubnamespaceSpec{ResourceQuotaSpec: corev1.ResourceQuotaSpec{
			Hard: composeQuota(defaultQuantity),
		}},
	}
}

func getChildNs(subnamespace *danav1alpha1.Subnamespace) corev1.Namespace {
	return corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", subnamespace.Namespace, subnamespace.Name),
		},
	}
}

func getRbForNs(binding *rbacv1.RoleBinding, namespace *corev1.Namespace) rbacv1.RoleBinding {
	return rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      binding.Name,
			Namespace: namespace.Name,
		}}
}

func getCrqForNs(namespace *corev1.Namespace) qoutav1.ClusterResourceQuota {
	return qoutav1.ClusterResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace.Name,
		},
	}
}

func isCreated(ctx context.Context, k8sClient client.Client, obj client.Object) {
	EventuallyWithOffset(1, func() bool {
		if err := k8sClient.Get(ctx, client.ObjectKey{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		}, obj); err != nil {
			return false
		}
		return true
	}, timeout, interval).Should(BeTrue())
}

func isDeleted(ctx context.Context, k8sClient client.Client, obj client.Object) {
	EventuallyWithOffset(1, func() bool {
		if err := k8sClient.Get(ctx, client.ObjectKey{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		}, obj); err != nil {
			if apierrors.IsNotFound(err) {
				return true
			}
		}
		return false
	}, timeout, interval).Should(BeTrue())

}
