// Copyright 2020 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package razeedeployment

import (
	"time"

	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/test/rectest"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Testing with Ginkgo", func() {
	It("razee deploy controller", func() {

		// TestMeterBaseController runs ReconcileMemcached.Reconcile() against a
		// fake client that tracks a MeterBase object.
		scheme.Scheme.AddKnownTypes(marketplacev1alpha1.SchemeGroupVersion, razeeDeployment.DeepCopy(), &marketplacev1alpha1.RazeeDeploymentList{}, &marketplacev1alpha1.RemoteResourceS3{}, &marketplacev1alpha1.RemoteResourceS3List{})
		testCleanInstall(GinkgoT())
		testNoSecret(GinkgoT())
		testBadName(GinkgoT())
		testFullUninstall(GinkgoT())
		testLegacyUninstall(GinkgoT())
	})
})

func newUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
			"spec": name,
		},
	}
}

func setup(r *ReconcilerTest) error {
	r.SetClient(fake.NewFakeClient(r.GetGetObjects()...))
	r.SetReconciler(&ReconcileRazeeDeployment{client: r.GetClient(), scheme: scheme.Scheme, opts: &RazeeOpts{RhmRRS3DeploymentImage: "rhm-rrs3-deployment-image", RhmWatchKeeperImage: "rhm-watch-keeper-image"}})
	return nil
}

var (
	name       = utils.RAZEE_NAME
	namespace  = "redhat-marketplace"
	secretName = "rhm-operator-secret"

	opts = []StepOption{
		WithRequest(req),
	}
	req = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
	razeeDeployment = marketplacev1alpha1.RazeeDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.RazeeDeploymentSpec{
			Enabled:               true,
			ClusterUUID:           "foo",
			DeploySecretName:      &secretName,
			LegacyUninstallHasRun: ptr.Bool(true),
		},
	}
	razeeDeploymentLegacyUninstall = marketplacev1alpha1.RazeeDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.RazeeDeploymentSpec{
			Enabled:          true,
			ClusterUUID:      "foo",
			DeploySecretName: &secretName,
		},
	}
	razeeDeploymentDeletion = marketplacev1alpha1.RazeeDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			DeletionTimestamp: &metav1.Time{Time: time.Now()},
		},
		Spec: marketplacev1alpha1.RazeeDeploymentSpec{
			Enabled:          true,
			ClusterUUID:      "foo",
			DeploySecretName: &secretName,
			TargetNamespace:  &namespace,
		},
	}

	namespObj = corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	razeeNsObj = corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "razee",
		},
	}
	console = &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "config.openshift.io/v1",
			"kind":       "Console",
			"metadata": map[string]interface{}{
				"name": "cluster",
			},
			"spec": "console",
		},
	}
	cluster = &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "config.openshift.io/v1",
			"kind":       "Infrastructure",
			"metadata": map[string]interface{}{
				"name": "cluster",
			},
			"spec": "console",
		},
	}
	clusterVersion = &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "config.openshift.io/v1",
			"kind":       "ClusterVersion",
			"metadata": map[string]interface{}{
				"name": "version",
			},
			"spec": "console",
		},
	}

	secret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rhm-operator-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			utils.IBM_COS_READER_KEY_FIELD: []byte("rhm-cos-reader-key"),
			utils.IBM_COS_URL_FIELD:        []byte("rhm-cos-url"),
			utils.BUCKET_NAME_FIELD:        []byte("bucket-name"),
			utils.RAZEE_DASH_ORG_KEY_FIELD: []byte("razee-dash-org-key"),
			utils.CHILD_RRS3_YAML_FIELD:    []byte("childRRS3-filename"),
			utils.RAZEE_DASH_URL_FIELD:     []byte("razee-dash-url"),
			utils.FILE_SOURCE_URL_FIELD:    []byte("file-source-url"),
		},
	}

	razeeJob = batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.RAZEE_DEPLOY_JOB_NAME,
			Namespace: namespace,
		},
		Spec: batch.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: utils.RAZEE_SERVICE_ACCOUNT,
				},
			},
		},
	}
	cosReaderKeySecret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.COS_READER_KEY_NAME,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			utils.IBM_COS_READER_KEY_FIELD: []byte("rhm-cos-reader-key"),
		},
	}
	configMap = corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.WATCH_KEEPER_CONFIG_NAME,
			Namespace: namespace,
		},
	}
	serviceAccount = corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "razeedeploy-sa",
			Namespace: namespace,
		},
	}
	clusterRole = rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redhat-marketplace-operator",
			Namespace: namespace,
		},
	}
	deployment = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.RHM_WATCHKEEPER_DEPLOYMENT_NAME,
			Namespace: namespace,
		},
	}
	parentRRS3 = marketplacev1alpha1.RemoteResourceS3{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.PARENT_RRS3_RESOURCE_NAME,
			Namespace: namespace,
		},
	}
)

func testFullUninstall(t GinkgoTInterface) {
	t.Parallel()

	reconcilerTest := NewReconcilerTest(setup,
		&razeeDeploymentDeletion,
		&parentRRS3,
		&cosReaderKeySecret,
		&configMap,
		&deployment,
	)

	reconcilerTest.TestAll(t,
		ReconcileStep(opts,
			ReconcileWithUntilDone(true)),
		ListStep(opts,
			ListWithObj(&marketplacev1alpha1.RemoteResourceS3List{}),
			ListWithFilter(
				client.InNamespace(namespace),
			),
			ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
				list, ok := i.(*marketplacev1alpha1.RemoteResourceS3List)
				assert.Truef(t, ok, "expected RemoteResourceS3List got type %T", i)
				assert.Equal(t, 0, len(list.Items))
			})),
		ListStep(opts,
			ListWithObj(&corev1.ConfigMapList{}),
			ListWithFilter(
				client.InNamespace(namespace),
			),
			ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
				list, ok := i.(*corev1.ConfigMapList)
				assert.Truef(t, ok, "expected configMap list got type %T", i)
				assert.Equal(t, 0, len(list.Items))
			})),
		ListStep(opts,
			ListWithObj(&corev1.SecretList{}),
			ListWithFilter(
				client.InNamespace(namespace),
			),
			ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
				list, ok := i.(*corev1.SecretList)

				assert.Truef(t, ok, "expected secret list got type %T", i)
				assert.Equal(t, 0, len(list.Items))
			})),
		ListStep(opts,
			ListWithObj(&appsv1.DeploymentList{}),
			ListWithFilter(
				client.InNamespace(namespace),
			),
			ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
				list, ok := i.(*appsv1.DeploymentList)

				assert.Truef(t, ok, "expected deployment list got type %T", i)
				assert.Equal(t, 0, len(list.Items))
			})),
	)
}

func testLegacyUninstall(t GinkgoTInterface) {
	t.Parallel()

	reconcilerTest := NewReconcilerTest(setup,
		&razeeDeploymentLegacyUninstall,
		&secret,
		&namespObj,
	)

	reconcilerTest.TestAll(t,
		ReconcileStep(opts,
			ReconcileWithExpectedResults(
				RequeueResult,
				RequeueResult,
				RequeueResult,
			),
		),
		ListStep(append(opts, WithStepName("Get Legacy Job")),
			ListWithObj(&batch.JobList{}),
			ListWithFilter(
				client.InNamespace(namespace),
			),
			ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
				list, ok := i.(*batch.JobList)

				assert.Truef(t, ok, "expected job list got type %T", i)
				assert.Equal(t, 0, len(list.Items))
			})),
		ListStep(append(opts, WithStepName("Get Service Account")),
			ListWithObj(&corev1.ServiceAccountList{}),
			ListWithFilter(
				client.InNamespace(namespace),
			),
			ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
				list, ok := i.(*corev1.ServiceAccountList)

				assert.Truef(t, ok, "expected service account list got type %T", i)
				assert.Equal(t, 0, len(list.Items))
			})),
		ListStep(append(opts, WithStepName("Get Cluster Role")),
			ListWithObj(&rbacv1.ClusterRoleList{}),
			ListWithFilter(
				client.InNamespace(namespace),
			),
			ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
				list, ok := i.(*rbacv1.ClusterRoleList)

				assert.Truef(t, ok, "expected cluster role list got type %T", i)
				assert.Equal(t, 0, len(list.Items))
			})),
		ListStep(append(opts, WithStepName("Get Deployment")),
			ListWithObj(&appsv1.DeploymentList{}),
			ListWithFilter(
				client.InNamespace(namespace),
			),
			ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
				list, ok := i.(*appsv1.DeploymentList)

				assert.Truef(t, ok, "expected deployment list got type %T", i)
				assert.Equal(t, 0, len(list.Items))
			})),
	)
}

func testCleanInstall(t GinkgoTInterface) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup,
		&razeeDeployment,
		&secret,
		&namespObj,
		console,
		cluster,
		clusterVersion,
	)
	reconcilerTest.TestAll(t,
		ReconcileStep(opts,
			ReconcileWithExpectedResults(
				append(
					RangeReconcileResults(RequeueResult, 15))...)),
		// Let's do some client checks
		ListStep(opts,
			ListWithObj(&corev1.ConfigMapList{}),
			ListWithFilter(
				client.InNamespace(namespace),
			),
			ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
				list, ok := i.(*corev1.ConfigMapList)

				assert.Truef(t, ok, "expected operator group list got type %T", i)
				assert.Equal(t, 4, len(list.Items))

				names := []string{}
				for _, cm := range list.Items {
					names = append(names, cm.Name)
				}

				assert.Contains(t, names, utils.WATCH_KEEPER_NON_NAMESPACED_NAME)
				assert.Contains(t, names, utils.WATCH_KEEPER_LIMITPOLL_NAME)
				assert.Contains(t, names, utils.WATCH_KEEPER_CONFIG_NAME)
				assert.Contains(t, names, utils.RAZEE_CLUSTER_METADATA_NAME)
			})),
		ListStep(opts,
			ListWithObj(&corev1.SecretList{}),
			ListWithFilter(
				client.InNamespace(namespace),
			),
			ListWithCheckResult(func(r *ReconcilerTest, t ReconcileTester, i runtime.Object) {
				list, ok := i.(*corev1.SecretList)

				assert.Truef(t, ok, "expected operator group list got type %T", i)
				assert.Equal(t, 3, len(list.Items))

				names := []string{}
				for _, cm := range list.Items {
					names = append(names, cm.Name)
				}

				assert.Contains(t, names, utils.WATCH_KEEPER_SECRET_NAME)
				assert.Contains(t, names, utils.RHM_OPERATOR_SECRET_NAME)
				assert.Contains(t, names, utils.COS_READER_KEY_NAME)
			})),
		ReconcileStep(opts,
			ReconcileWithUntilDone(true)),
	)
}

var (
	oldRazeeDeployment = marketplacev1alpha1.RazeeDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: marketplacev1alpha1.RazeeDeploymentSpec{
			Enabled:     true,
			ClusterUUID: "foo",
		},
		Status: marketplacev1alpha1.RazeeDeploymentStatus{
			RazeeJobInstall: &marketplacev1alpha1.RazeeJobInstallStruct{
				RazeeNamespace: "razee",
			},
		},
	}
)

func testNoSecret(t GinkgoTInterface) {
	t.Parallel()
	reconcilerTest := NewReconcilerTest(setup, &razeeDeployment, &namespObj)
	reconcilerTest.TestAll(t,
		ReconcileStep(opts,
			ReconcileWithExpectedResults(
				RequeueResult,
				RequeueResult,
				RequeueResult,
				RequeueAfterResult(time.Second*60)),
		))
}

func testBadName(t GinkgoTInterface) {
	t.Parallel()
	razeeDeploymentLocalDeployment := razeeDeployment.DeepCopy()
	razeeDeploymentLocalDeployment.Name = "foo"
	reconcilerTest := NewReconcilerTest(setup, razeeDeploymentLocalDeployment, &namespObj)
	reconcilerTest.TestAll(t,
		ReconcileStep(opts,
			ReconcileWithExpectedResults(
				DoneResult,
			),
		))
}

func CreateWatchKeeperSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "razeedeploy-job",
			Namespace: "marketplace-operator",
		},
	}
}
