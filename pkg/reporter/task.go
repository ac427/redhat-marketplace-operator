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

package reporter

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"bytes"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/gotidy/ptr"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/log"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/managers"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	"github.com/redhat-marketplace/redhat-marketplace-operator/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/jsonpath"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Task struct {
	ReportName ReportName

	CC        ClientCommandRunner
	Cache     cache.Cache
	K8SClient client.Client
	Ctx       context.Context
	Config    *Config
	K8SScheme *runtime.Scheme
	Uploader  *RedHatInsightsUploader
}

func (r *Task) Run() error {
	logger.Info("task run start")
	stopCh := make(chan struct{})
	defer close(stopCh)

	r.Cache.WaitForCacheSync(stopCh)

	logger.Info("creating reporter job")
	reporter, err := NewReporter(r)

	if err != nil {
		return err
	}

	logger.Info("starting collection")
	metrics, errorList, err := reporter.CollectMetrics(r.Ctx)

	if err != nil {
		logger.Error(err, "error collecting metrics")
		return err
	}

	reportID := uuid.New()

	logger.Info("writing report", "reportID", reportID)

	files, err := reporter.WriteReport(
		reportID,
		metrics)

	if err != nil {
		return errors.Wrap(err, "error writing report")
	}

	dirpath := filepath.Dir(files[0])
	fileName := fmt.Sprintf("%s/../upload-%s.tar.gz", dirpath, reportID.String())
	err = TargzFolder(dirpath, fileName)

	logger.Info("tarring", "outputfile", fileName)

	if r.Config.Upload {
		err = r.Uploader.UploadFile(fileName)

		if err != nil {
			return errors.Wrap(err, "error uploading file")
		}

		logger.Info("uploaded metrics", "metrics", len(metrics))
	}

	report := &marketplacev1alpha1.MeterReport{}
	err = utils.Retry(func() error {
		result, _ := r.CC.Do(
			r.Ctx,
			HandleResult(
				GetAction(types.NamespacedName(r.ReportName), report),
				OnContinue(Call(func() (ClientAction, error) {
					report.Status.MetricUploadCount = ptr.Int(len(metrics))

					report.Status.QueryErrorList = []string{}

					for _, err := range errorList {
						report.Status.QueryErrorList = append(report.Status.QueryErrorList, err.Error())
					}

					return UpdateAction(report), nil
				})),
			),
		)

		if result.Is(Error) {
			return result
		}

		return nil
	}, 3)

	if err != nil {
		log.Error(err, "failed to update report")
	}

	return nil
}

func provideApiClient(
	report *marketplacev1alpha1.MeterReport,
	promService *corev1.Service,
	config *Config,
) (api.Client, error) {

	if config.Local {
		client, err := api.NewClient(api.Config{
			Address: "http://localhost:9090",
		})

		if err != nil {
			return nil, err
		}

		return client, nil
	}

	var port int32
	name := promService.Name
	namespace := promService.Namespace
	targetPort := report.Spec.PrometheusService.TargetPort

	switch {
	case targetPort.Type == intstr.Int:
		port = targetPort.IntVal
	default:
		for _, p := range promService.Spec.Ports {
			if p.Name == targetPort.StrVal {
				port = p.Port
			}
		}
	}

	var auth = ""
	if config.TokenFile != "" {
		content, err := ioutil.ReadFile(config.TokenFile)
		if err != nil {
			return nil, err
		}
		auth = fmt.Sprintf(string(content))
	}

	conf, err := NewSecureClient(&PrometheusSecureClientConfig{
		Address:        fmt.Sprintf("https://%s.%s.svc:%v", name, namespace, port),
		ServerCertFile: config.CaFile,
		Token:          auth,
	})

	if err != nil {
		return nil, err
	}

	return conf, nil
}

func getClientOptions() managers.ClientOptions {
	return managers.ClientOptions{
		Namespace:    "",
		DryRunClient: false,
	}
}

func provideProductionInsights(
	ctx context.Context,
	cc ClientCommandRunner,
	log logr.Logger,
	isCacheStarted managers.CacheIsStarted,
) (*RedHatInsightsUploaderConfig, error) {
	secret := &corev1.Secret{}
	clusterVersion := &openshiftconfigv1.ClusterVersion{}
	result, _ := cc.Do(ctx,
		GetAction(types.NamespacedName{
			Name:      "pull-secret",
			Namespace: "openshift-config",
		}, secret),
		GetAction(types.NamespacedName{
			Name: "version",
		}, clusterVersion))

	if !result.Is(Continue) {
		return nil, result
	}

	dockerConfigBytes, ok := secret.Data[".dockerconfigjson"]

	if !ok {
		return nil, errors.New(".dockerconfigjson is not found in secret")
	}

	var dockerObj interface{}
	err := json.Unmarshal(dockerConfigBytes, &dockerObj)

	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal dockerConfigJson object")
	}

	cloudAuthPath := jsonpath.New("cloudauthpath")
	err = cloudAuthPath.Parse(`{.auths.cloud\.openshift\.com.auth}`)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get jsonpath of cloud token")
	}

	buf := new(bytes.Buffer)
	err = cloudAuthPath.Execute(buf, dockerObj)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get jsonpath of cloud token")
	}

	cloudToken := buf.String()

	return &RedHatInsightsUploaderConfig{
		URL:             "https://cloud.redhat.com",
		ClusterID:       string(clusterVersion.Spec.ClusterID), // get from cluster
		OperatorVersion: version.Version,
		Token:           cloudToken, // get from secret
	}, nil
}

func getMarketplaceConfig(
	ctx context.Context,
	cc ClientCommandRunner,
) (config *marketplacev1alpha1.MarketplaceConfig, returnErr error) {
	config = &marketplacev1alpha1.MarketplaceConfig{}

	if result, _ := cc.Do(ctx,
		GetAction(
			types.NamespacedName{Namespace: "openshift-redhat-marketplace", Name: utils.MARKETPLACECONFIG_NAME}, config,
		)); !result.Is(Continue) {
		returnErr = errors.Wrap(result, "failed to get mkplc config")
	}

	logger.Info("retrieved meter report")
	return
}

func getMarketplaceReport(
	ctx context.Context,
	cc ClientCommandRunner,
	reportName ReportName,
) (report *marketplacev1alpha1.MeterReport, returnErr error) {
	report = &marketplacev1alpha1.MeterReport{}

	if result, _ := cc.Do(ctx, GetAction(types.NamespacedName(reportName), report)); !result.Is(Continue) {
		returnErr = errors.Wrap(result, "failed to get report")
	}

	logger.Info("retrieved meter report")
	return
}

func getPrometheusService(
	ctx context.Context,
	report *marketplacev1alpha1.MeterReport,
	cc ClientCommandRunner,
) (service *corev1.Service, returnErr error) {
	service = &corev1.Service{}

	if report.Spec.PrometheusService == nil {
		returnErr = errors.New("cannot retrieve service as the report doesn't have a value for it")
		return
	}

	name := types.NamespacedName{
		Name:      report.Spec.PrometheusService.Name,
		Namespace: report.Spec.PrometheusService.Namespace,
	}

	if result, _ := cc.Do(ctx, GetAction(name, service)); !result.Is(Continue) {
		returnErr = errors.Wrap(result, "failed to get report")
	}

	logger.Info("retrieved prometheus service")
	return
}

func getMeterDefinitions(
	ctx context.Context,
	report *marketplacev1alpha1.MeterReport,
	cc ClientCommandRunner,
) ([]marketplacev1alpha1.MeterDefinition, error) {
	defs := &marketplacev1alpha1.MeterDefinitionList{}

	if len(report.Spec.MeterDefinitions) > 0 {
		return report.Spec.MeterDefinitions, nil
	}

	result, _ := cc.Do(ctx,
		HandleResult(
			ListAction(defs, client.InNamespace("")),
			OnContinue(Call(func() (ClientAction, error) {
				for _, item := range defs.Items {
					item.Status = marketplacev1alpha1.MeterDefinitionStatus{}
				}

				report.Spec.MeterDefinitions = defs.Items

				return UpdateAction(report), nil
			})),
		),
	)

	if result.Is(NotFound) {
		return []marketplacev1alpha1.MeterDefinition{}, nil
	}

	if !result.Is(Continue) {
		return nil, errors.Wrap(result, "failed to get meterdefs")
	}

	return defs.Items, nil
}
