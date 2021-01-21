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
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1api "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	rbacv1 "k8s.io/api/rbac/v1"
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
	_ = context.Background()
	log := r.Log.WithValues("rolebinding", req.NamespacedName)
	log.Info("starting to reconcile")
	var (
		rb      rbacv1.RoleBinding
		ns      corev1.Namespace
		kind    string
		snsList danav1alpha1.SubnamespaceList
		//ishns string
	)
	// get namespace name to ns
	if err := r.Get(ctx, client.ObjectKey{Namespace: "", Name: req.Namespace}, &ns); err != nil {
		log.Error(err, "Could not find  Namespace ")
		return ctrl.Result{}, err
	}
	/*
		ishns = ns.ObjectMeta.Labels[danav1alpha1.Hns]
		if ishns == ""{
			log.Info("no hns")
			return  ctrl.Result{}, nil
		}
	*/
	// get the rb
	if err := r.Get(ctx, req.NamespacedName, &rb); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		log.Error(err, "unable to get rb")
		return ctrl.Result{}, err
	}
	//create a subnamespaces list
	if err := r.List(ctx, &snsList, client.InNamespace(ns.Name)); err != nil {
		log.Info("FAILED TO LIST SNS")
		return ctrl.Result{}, err
	}

	kind = rb.Subjects[0].Kind

	if kind != "User" {
		return ctrl.Result{}, nil
	}

	if shouldCleanUp2(&rb) {
		log.Info("RB delete")
		return ctrl.Result{}, r.DeleteRB(ctx, log, &rb, snsList)
	}

	if FinalizerCheck(&rb) {
		if err := r.CreateFinalzier(ctx, log, &rb); err != nil {
			return ctrl.Result{}, err
		}
	}
	//if the recocile is not for deleting and dont have finzilizer , lets create it
	log.Info("RB CREATE WILL START ")
	if err := r.CreateRB(ctx, log, &rb, snsList); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *RoleBindingReconciler) DeleteRB(ctx context.Context, log logr.Logger, rb *rbacv1.RoleBinding, snsList danav1alpha1.SubnamespaceList) error {
	var (
		rbtodelete    rbacv1.RoleBinding
		namespacename string
	)
	for _, sns := range snsList.Items {
		namespacename = sns.ObjectMeta.Annotations[danav1alpha1.Pointer]
		rbtodelete = buildRbs(rb, namespacename)
		if err := r.Delete(ctx, &rbtodelete); err != nil {
			if !apierrors.IsNotFound(err) {
				log.V(4).Error(err, "unable to delete RoleBinding")
				return err
			}
		}
	}
	//delete the rb
	controllerutil.RemoveFinalizer(rb, danav1alpha1.RbFinalizer)
	if err := r.Update(ctx, rb); err != nil {
		log.V(4).Error(err, "unable to update RoleBinding")
		return err
	}
	return nil

}

func (r *RoleBindingReconciler) CreateRB(ctx context.Context, log logr.Logger, rb *rbacv1.RoleBinding, snsList danav1alpha1.SubnamespaceList) error {
	var (
		rbtocreate    rbacv1.RoleBinding
		namespacename string
	)

	//create the rb
	for _, sns := range snsList.Items {
		namespacename = sns.ObjectMeta.Annotations[danav1alpha1.Pointer]
		if err := r.Get(ctx, client.ObjectKey{Namespace: namespacename, Name: rb.Name}, &rbtocreate); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Info("resource already exist")
				return err
			}
		}
		if rbtocreate.Name != "" {
			continue
		}
		rbtocreate = buildRbs(rb, namespacename)
		if err := r.Create(ctx, &rbtocreate); err != nil {
			if !apierrors.IsConflict(err) {
				log.V(4).Error(err, "unable to Create RoleBinding")
				return err
			}
		}
	}

	return nil

}

//build rb object
func buildRbs(rb *rbacv1.RoleBinding, namespace string) rbacv1.RoleBinding {
	return rbacv1.RoleBinding{
		ObjectMeta: v1api.ObjectMeta{
			Name:      rb.Name,
			Namespace: namespace,
		},
		Subjects: rb.Subjects,
		RoleRef:  rb.RoleRef,
	}
}

func (r *RoleBindingReconciler) CreateFinalzier(ctx context.Context, log logr.Logger, rb *rbacv1.RoleBinding) error {

	controllerutil.AddFinalizer(rb, danav1alpha1.RbFinalizer)
	if err := r.Update(ctx, rb); err != nil {
		log.V(4).Error(err, "unable to update namespace")
		return err
	}

	return nil
}

func FinalizerCheck(rb *rbacv1.RoleBinding) bool {
	return !controllerutil.ContainsFinalizer(rb, danav1alpha1.RbFinalizer)
}

func shouldCleanUp2(rb *rbacv1.RoleBinding) bool {
	return !rb.DeletionTimestamp.IsZero()
}

func (r *RoleBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rbacv1.RoleBinding{}).
		Complete(r)
}
