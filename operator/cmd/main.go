// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourcev1b2 "github.com/fluxcd/source-controller/api/v1beta2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
	autocontroller "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/internal/controller/auto"
	pulumicontroller "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/internal/controller/pulumi"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/version"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(sourcev1.AddToScheme(scheme))
	utilruntime.Must(sourcev1b2.AddToScheme(scheme))
	utilruntime.Must(autov1alpha1.AddToScheme(scheme))
	utilruntime.Must(pulumiv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var (
		metricsAddr          string
		enableLeaderElection bool
		probeAddr            string
		secureMetrics        bool
		enableHTTP2          bool

		// Flags for configuring the Program file server.
		programFSAddr    string
		programFSAdvAddr string

		// Flags for configuring leader election timeouts.
		leaderElectionLeaseDuration time.Duration
		leaderElectionRenewDeadline time.Duration
		leaderElectionRetryPeriod   time.Duration
		kubeAPITimeout              time.Duration
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metric endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&programFSAddr, "program-fs-addr", envOrDefault("PROGRAM_FS_ADDR", ":9090"),
		"The address the static file server binds to.")
	flag.StringVar(&programFSAdvAddr, "program-fs-adv-addr", envOrDefault("PROGRAM_FS_ADV_ADDR", ""),
		"The advertised address of the static file server.")
	flag.DurationVar(&leaderElectionLeaseDuration, "leader-election-lease-duration", 60*time.Second,
		"Duration that non-leader candidates will wait to force acquire leadership. "+
			"Can also be set via LEADER_ELECTION_LEASE_DURATION environment variable.")
	flag.DurationVar(&leaderElectionRenewDeadline, "leader-election-renew-deadline", 45*time.Second,
		"Duration the leader will retry refreshing leadership before giving up. "+
			"Can also be set via LEADER_ELECTION_RENEW_DEADLINE environment variable.")
	flag.DurationVar(&leaderElectionRetryPeriod, "leader-election-retry-period", 10*time.Second,
		"Duration the LeaderElector clients should wait between tries of actions. "+
			"Can also be set via LEADER_ELECTION_RETRY_PERIOD environment variable.")
	flag.DurationVar(&kubeAPITimeout, "kube-api-timeout", 30*time.Second,
		"Timeout for requests to the Kubernetes API server. "+
			"Can also be set via KUBE_API_TIMEOUT environment variable.")

	// Configure Zap-based logging for klog/v2 and for the controller manager.
	// Write to stdout by default.
	opts := zap.Options{
		DestWriter: os.Stdout,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	logger := zap.New(zap.UseFlagOptions(&opts))
	klog.SetLogger(logger)
	ctrllog.SetLogger(logger)

	setupLog.Info("Pulumi Kubernetes Operator Manager",
		"version", version.Version,
	)

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancelation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		// TODO(user): TLSOpts is used to allow configuring the TLS config used for the server. If certificates are
		// not provided, self-signed certificates will be generated by default. This option is not recommended for
		// production environments as self-signed certificates do not offer the same level of trust and security
		// as certificates issued by a trusted Certificate Authority (CA). The primary risk is potentially allowing
		// unauthorized access to sensitive metrics data. Consider replacing with CertDir, CertName, and KeyName
		// to provide certificates, ensuring the server communicates using trusted and secure certificates.
		TLSOpts: tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// Override leader election timeouts from environment variables if set
	if s, ok := os.LookupEnv("LEADER_ELECTION_LEASE_DURATION"); ok {
		if d, err := time.ParseDuration(s); err == nil {
			leaderElectionLeaseDuration = d
		}
	}
	if s, ok := os.LookupEnv("LEADER_ELECTION_RENEW_DEADLINE"); ok {
		if d, err := time.ParseDuration(s); err == nil {
			leaderElectionRenewDeadline = d
		}
	}
	if s, ok := os.LookupEnv("LEADER_ELECTION_RETRY_PERIOD"); ok {
		if d, err := time.ParseDuration(s); err == nil {
			leaderElectionRetryPeriod = d
		}
	}
	if s, ok := os.LookupEnv("KUBE_API_TIMEOUT"); ok {
		if d, err := time.ParseDuration(s); err == nil {
			kubeAPITimeout = d
		}
	}

	controllerOpts := config.Controller{
		MaxConcurrentReconciles: 25,
	}
	if s, ok := os.LookupEnv("MAX_CONCURRENT_RECONCILES"); ok {
		controllerOpts.MaxConcurrentReconciles, _ = strconv.Atoi(s)
	}

	// Configure REST client with custom timeout
	restConfig := ctrl.GetConfigOrDie()
	restConfig.Timeout = kubeAPITimeout

	setupLog.Info("Configured Kubernetes API client",
		"timeout", kubeAPITimeout,
	)

	if enableLeaderElection {
		setupLog.Info("Configured leader election",
			"leaseDuration", leaderElectionLeaseDuration,
			"renewDeadline", leaderElectionRenewDeadline,
			"retryPeriod", leaderElectionRetryPeriod,
		)
	}

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		Controller:             controllerOpts,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "operator.pulumi.com",
		LeaseDuration:          &leaderElectionLeaseDuration,
		RenewDeadline:          &leaderElectionRenewDeadline,
		RetryPeriod:            &leaderElectionRetryPeriod,
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader doesn't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you intend to do any operation such as perform cleanups after the
		// manager stops then its usage might be unsafe.
		LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Create a new ProgramHandler to handle Program objects. Both the ProgramReconciler and the file server need to
	// access the ProgramHandler, so it is created here and passed to both.
	if programFSAdvAddr == "" {
		programFSAdvAddr = determineAdvAddr(programFSAddr)
	}
	pHandler := pulumicontroller.NewProgramHandler(mgr.GetClient(), programFSAdvAddr)

	// Create a connection manager for making authenticated connections to workspaces.
	cm, err := autocontroller.NewConnectionManager(restConfig, autocontroller.ConnectionManagerOptions{
		ServiceAccount: types.NamespacedName{
			Namespace: envOrDefault("POD_NAMESPACE", "pulumi-kubernetes-operator"),
			Name:      envOrDefault("POD_SA_NAME", "controller-manager"),
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to create connection manager")
		os.Exit(1)
	}
	if err := mgr.Add(cm); err != nil {
		setupLog.Error(err, "unable to add connection manager")
		os.Exit(1)
	}

	if err = (&autocontroller.WorkspaceReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		Recorder:          mgr.GetEventRecorderFor("workspace-controller"),
		ConnectionManager: cm,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Workspace")
		os.Exit(1)
	}
	if err = (&autocontroller.UpdateReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		Recorder:          mgr.GetEventRecorderFor("update-controller"),
		ConnectionManager: cm,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Update")
		os.Exit(1)
	}
	if err = (&pulumicontroller.StackReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("stack-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Stack")
		os.Exit(1)
	}
	if err = (&pulumicontroller.ProgramReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		Recorder:       mgr.GetEventRecorderFor(pulumicontroller.ProgramControllerName),
		ProgramHandler: pHandler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Program")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Start the file server for serving Program objects.
	setupLog.Info("starting file server for program resource",
		"address", programFSAddr,
		"advertisedAddress", programFSAdvAddr,
	)
	if err := mgr.Add(pFileserver{pHandler, programFSAddr}); err != nil {
		setupLog.Error(err, "unable to start file server for program resource")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// pFileserver implements the manager.Runnable interface to start a simple file server to serve Program objects as
// compressed tarballs.
type pFileserver struct {
	handler *pulumicontroller.ProgramHandler
	address string
}

// Start starts the file server to serve Program objects as compressed tarballs.
func (fs pFileserver) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.Handle("/programs/", fs.handler.HandleProgramServing())

	server := &http.Server{
		Addr:              fs.address,
		Handler:           mux,
		ReadHeaderTimeout: 0,
	}

	errChan := make(chan error)
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return server.Shutdown(ctx)
	}
}

func determineAdvAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		setupLog.Error(err, "unable to parse file server address")
		os.Exit(1)
	}
	switch host {
	case "":
		host = "localhost"
	case "0.0.0.0":
		host = os.Getenv("HOSTNAME")
		if host == "" {
			hn, err := os.Hostname()
			if err != nil {
				setupLog.Error(err, "0.0.0.0 specified in file server addr but hostname is invalid")
				os.Exit(1)
			}
			host = hn
		}
	}
	return net.JoinHostPort(host, port)
}

func envOrDefault(envName, defaultValue string) string {
	ret := os.Getenv(envName)
	if ret != "" {
		return ret
	}

	return defaultValue
}
