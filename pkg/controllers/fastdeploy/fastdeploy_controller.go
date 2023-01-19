/*
Copyright 2023 mark.

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
	"bytes"
	"context"
	"embed"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"text/template"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appv1 "github.com/mark8s/fastdeploy/pkg/apis/fastdeploy/v1"
	corev1 "k8s.io/api/core/v1"
)

// FastDeployReconciler reconciles a FastDeploy object
type FastDeployReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//go:embed template
var f embed.FS

//+kubebuilder:rbac:groups=app.mark8s.io,resources=fastdeploys,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=app.mark8s.io,resources=fastdeploys/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=app.mark8s.io,resources=fastdeploys/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the FastDeploy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.1/pkg/reconcile
func (r *FastDeployReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Log.Info("Reconcile: " + req.NamespacedName.String())
	_ = log.FromContext(ctx)
	build := &appv1.FastDeploy{}
	err := r.Get(ctx, req.NamespacedName, build)
	if err != nil {
		log.Log.Error(err, "Get FastDeploy error")
		return ctrl.Result{}, err
	}

	// 查找缓存中 同key 的 deployment 和 service
	deploy := &v1.Deployment{}
	deployment := r.buildDeployment(build)
	// 建立关联后，删除fastdeploy资源时就会将deploy删除掉
	err = controllerutil.SetOwnerReference(build, deployment, r.Scheme)
	if err != nil {
		log.Log.Error(err, "SetOwnerReference error")
		return ctrl.Result{}, err
	}

	err = r.Get(ctx, types.NamespacedName{Name: build.Spec.Name, Namespace: build.Spec.Namespace}, deploy)
	if err != nil {
		// 不存在deploy就得创建
		if errors.IsNotFound(err) {
			err = r.Create(ctx, deployment)
			if err != nil {
				return ctrl.Result{}, err
			}
			log.Log.Info("Create deployment: " + deployment.Name + " on namespace: " + deployment.Namespace)
		}
	} else {
		if err := r.Update(ctx, deployment); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 建立关联后，删除fastdeploy资源时就会将service也删除掉
	svc := &corev1.Service{}
	service := r.buildService(build)
	err = controllerutil.SetOwnerReference(build, service, r.Scheme)
	if err != nil {
		log.Log.Error(err, "SetOwnerReference error")
		return ctrl.Result{}, err
	}

	err = r.Get(ctx, types.NamespacedName{Name: build.Spec.Name, Namespace: build.Spec.Namespace}, svc)
	if err != nil {
		if errors.IsNotFound(err) && build.Spec.EnableService {
			if err = r.Create(ctx, service); err != nil {
				log.Log.Error(err, "Create service failed")
				return ctrl.Result{}, err
			}
			log.Log.Info("Create service: " + service.Name + " on namespace: " + service.Namespace)
		}
	} else {
		if build.Spec.EnableService {
			svc.Spec.Ports = service.Spec.Ports
			if err = r.Update(ctx, svc); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			if err := r.Delete(ctx, svc); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// 更新 QuickBuild Status
	r.updateQBStatus(ctx, build)

	return ctrl.Result{}, nil
}

func (r *FastDeployReconciler) updateQBStatus(ctx context.Context, build *appv1.FastDeploy) {
	log.Log.Info("Update FastDeploy Status")
	deploy := &v1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: build.Spec.Name, Namespace: build.Spec.Namespace}, deploy)
	if err != nil {
		log.Log.Error(err, "Not Found Deploy: "+deploy.Name+" On Namespace: "+deploy.Namespace)
		return
	}
	// 先简单点判断
	if deploy.Status.ReadyReplicas == deploy.Status.Replicas {
		build.Status.Status = "AllReady"
	} else {
		build.Status.Status = "NotReady"
	}

	svc := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: build.Spec.Name, Namespace: build.Spec.Namespace}, svc)
	if err != nil {
		log.Log.Error(err, "Not Found Svc: "+svc.Name+" On Namespace: "+svc.Namespace)
		return
	}

	build.Status.ServiceIP = svc.Spec.ClusterIP
	err = r.Status().Update(ctx, build)
	if err != nil {
		log.Log.Error(err, "Update fastdeploy: "+build.Name+" on namespace: "+build.Namespace+" error")
		return
	}
}

func (r *FastDeployReconciler) buildService(build *appv1.FastDeploy) *corev1.Service {
	s := &corev1.Service{}
	err := yaml.Unmarshal(r.parseTemplate("service", build), s)
	if err != nil {
		log.Log.Error(err, "Build Service Error")
		return nil
	}
	return s
}

func (r *FastDeployReconciler) buildDeployment(build *appv1.FastDeploy) *v1.Deployment {
	d := &v1.Deployment{}
	err := yaml.Unmarshal(r.parseTemplate("deployment", build), d)
	if err != nil {
		log.Log.Error(err, "Build Deploy Error")
		return nil
	}
	return d
}

func (r *FastDeployReconciler) parseTemplate(templateName string, build *appv1.FastDeploy) []byte {
	fileBytes, err := f.ReadFile("template/" + templateName + ".yaml")
	if err != nil {
		log.Log.Error(err, "")
		return nil
	}

	tmpl, err := template.New("tpl").Parse(string(fileBytes))
	if err != nil {
		log.Log.Error(err, "")
		return nil
	}
	b := new(bytes.Buffer)
	err = tmpl.Execute(b, build)
	if err != nil {
		return nil
	}
	return b.Bytes()
}

// SetupWithManager sets up the controller with the Manager.
func (r *FastDeployReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1.FastDeploy{}).
		Owns(&v1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
