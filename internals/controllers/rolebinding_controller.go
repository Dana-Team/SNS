/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	//"strings"
	//"strconv"
	"context"
	danav1alpha1 "github.com/Dana-Team/SNS/api/v1alpha1"
	"github.com/Dana-Team/SNS/internals/webhooks"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1api "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// RoleBindingReconciler reconciles a RoleBinding object
type RoleBindingReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=rbac.dana.sns.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.dana.sns.io,resources=rolebindings/status,verbs=get;update;patch

func (r *RoleBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("rolebinding", req.NamespacedName)
	log.Info("starting to reconcile")
	var (
		rb      rbacv1.RoleBinding
		ns      corev1.Namespace
		SnsList danav1alpha1.SubnamespaceList
		//IsHns   string
		//ishns string
	)

	// get the rb
	if err := r.Get(ctx, req.NamespacedName, &rb); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		log.V(2).Error(err, "unable to get rb")
		return ctrl.Result{}, err
	}

	// get namespace name to ns
	if err := r.Get(ctx, client.ObjectKey{Namespace: "", Name: req.Namespace}, &ns); err != nil {
		log.V(2).Error(err, "Could not find  namespace ")
		return ctrl.Result{}, err
	}
	/*
		IsHns = ns.ObjectMeta.Labels[danav1alpha1.Hns]
		if IsHns == "" {
			return ctrl.Result{}, nil
		}
	*/
	//create a subnamespaces list
	if err := r.List(ctx, &SnsList, client.InNamespace(ns.Name)); err != nil {
		return ctrl.Result{}, err
	}

	if rb.Subjects[0].Kind != "User" {
		return ctrl.Result{}, nil
	}

	if ShouldCleanUp(&rb) {
		log.V(2).Info("rb delete")
		return ctrl.Result{}, r.rbDelete(ctx, log, &rb, &SnsList)
	}

	if FinalizerCheck(&rb) {
		return ctrl.Result{}, r.CreateFinalzier(ctx, log, &rb)

	}
	//if the recocile is not for deleting and dont have finzilizer , lets create it
	if err := r.rbCreate(ctx, log, &rb, &SnsList); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *RoleBindingReconciler) rbDelete(ctx context.Context, log logr.Logger, rb *rbacv1.RoleBinding, snsList *danav1alpha1.SubnamespaceList) error {
	var RbToDelete rbacv1.RoleBinding

	for _, sns := range snsList.Items {
		RbToDelete = buildRbs(rb, sns)
		if err := r.Delete(ctx, &RbToDelete); err != nil {
			if !apierrors.IsNotFound(err) {
				log.V(2).Error(err, "unable to delete rolebinding")
				return err
			}
		}
	}
	//delete the rb
	controllerutil.RemoveFinalizer(rb, danav1alpha1.RbFinalizer)
	if err := r.Update(ctx, rb); err != nil {
		log.V(2).Error(err, "unable to update rolebinding")
		return err
	}
	return nil

}

func (r *RoleBindingReconciler) rbCreate(ctx context.Context, log logr.Logger, rb *rbacv1.RoleBinding, snsList *danav1alpha1.SubnamespaceList) error {
	var (
		RbToCreate    rbacv1.RoleBinding
		NamespaceName string
	)

	//create the rb
	for _, sns := range snsList.Items {
		NamespaceName = sns.ObjectMeta.Annotations[danav1alpha1.Pointer]
		if err := r.Get(ctx, client.ObjectKey{Namespace: NamespaceName, Name: rb.Name}, &RbToCreate); err != nil {
			if !apierrors.IsNotFound(err) {
				log.V(3).Info("resource already exist")
				return err
			}
		}
		if RbToCreate.Name != "" {
			continue
		}
		RbToCreate = buildRbs(rb, sns)
		if err := r.Create(ctx, &RbToCreate); err != nil {
			if !apierrors.IsConflict(err) {
				log.V(2).Error(err, "unable to create rolebinding")
				return err
			}
		}
	}

	return nil

}

//build rb object
func buildRbs(rb *rbacv1.RoleBinding, sns danav1alpha1.Subnamespace) rbacv1.RoleBinding {
	var NamespaceName string
	NamespaceName = sns.ObjectMeta.Annotations[danav1alpha1.Pointer]
	return rbacv1.RoleBinding{
		ObjectMeta: v1api.ObjectMeta{
			Name:      rb.Name,
			Namespace: NamespaceName,
		},
		Subjects: rb.Subjects,
		RoleRef:  rb.RoleRef,
	}
}

func (r *RoleBindingReconciler) CreateFinalzier(ctx context.Context, log logr.Logger, rb *rbacv1.RoleBinding) error {

	controllerutil.AddFinalizer(rb, danav1alpha1.RbFinalizer)
	if err := r.Update(ctx, rb); err != nil {
		log.V(2).Error(err, "unable to update namespace")
		return err
	}

	return nil
}

func (r *RoleBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rbacv1.RoleBinding{}).
		Complete(r)
}
