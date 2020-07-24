package report

import (
	"context"
	"os"

	"emperror.dev/errors"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/apis/marketplace/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/pkg/reporter"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/pkg/utils/reconcileutils"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var log = logf.Log.WithName("reporter_report_cmd")

var name, namespace string

var ReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Run the report",
	Long:  `Runs the report. Takes it name and namespace as args`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("running the report command")
		outputDir := os.TempDir()
		cfg := reporter.Config{
			OutputDirectory: outputDir,
		}

		stopCh := make(chan struct{})
		defer close(stopCh)

		if name == "" || namespace == "" {
			log.Error(errors.New("name or namespace not provided"), "namespace or name not provided")
			os.Exit(1)
		}

		ctx := context.TODO()

		report, err := initializeMarketplaceReporter(
			ctx,
			reporter.ReportName{Namespace: namespace, Name: name},
			cfg,
			stopCh,
		)

		if err != nil {
			log.Error(err, "")
			os.Exit(1)
		}

		metrics, err := report.CollectMetrics(ctx)

		if err != nil {
			log.Error(err, "")
			os.Exit(1)
		}

		log.Info("metrics", "metrics", metrics)

		os.Exit(0)
	},
}

func init() {
	ReportCmd.Flags().StringVar(&name, "name", "", "name of the report")
	ReportCmd.Flags().StringVar(&namespace, "namespace", "", "namespace of the report")
}

func provideOptions(kscheme *runtime.Scheme) (*manager.Options, error) {
	return &manager.Options{
		Namespace: "",
		Scheme:    kscheme,
	}, nil
}

func getMarketplaceReport(
	ctx context.Context,
	cc ClientCommandRunner,
	reportName reporter.ReportName,
) (report *marketplacev1alpha1.MeterReport, returnErr error) {
	report = &marketplacev1alpha1.MeterReport{}

	if result, _ := cc.Do(ctx, GetAction(types.NamespacedName(reportName), report)); !result.Is(Continue) {
		returnErr = errors.Wrap(result, "failed to get report")
	}

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

	return
}

func getMeterDefinitions(
	report *marketplacev1alpha1.MeterReport,
) []*marketplacev1alpha1.MeterDefinitionSpec {
	return report.Spec.MeterDefinitions
}
