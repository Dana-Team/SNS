package controllers

import (
	danav1alpha1 "github.com/Dana-Team/SNS/api/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func FinalizerCheck(rb *rbacv1.RoleBinding) bool {
	return !controllerutil.ContainsFinalizer(rb, danav1alpha1.RbFinalizer)
}

func ShouldCleanUp(rb *rbacv1.RoleBinding) bool {
	return !rb.DeletionTimestamp.IsZero()
}
