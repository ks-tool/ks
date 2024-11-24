/*
 Copyright (c) 2024 Alexey Shulutkov <github@shulutkov.ru>

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

package k8s

import (
	"context"
	"fmt"
	"go.etcd.io/etcd/server/v3/embed"
	"k8s.io/klog/v2"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	etcdSocketPath = "/tmp/etcd.sock"

	defaultKubernetesDir  = "/etc/kubernetes"
	defaultEtcdDir        = "/var/lib/etcd"
	defaultServicesSubnet = "172.18.0.0/21"
	defaultPodSubnet      = "172.21.0.0/18"
	defaultClusterName    = "kubernetes"
	defaultDnsDomain      = "cluster.local"
)

type ControlPlain struct {
	AdvertiseAddress string
	BindPort         int
	ClusterName      string
	DNSDomain        string
	EtcdDataDir      string
	EtcdSocketPath   string
	CertificatesDir  string
	KubernetesDir    string
	PodSubnet        string
	ServiceSubnet    string

	APIServerExtraArgs         []kubeadmapi.Arg
	ControllerManagerExtraArgs []kubeadmapi.Arg
	SchedulerExtraArgs         []kubeadmapi.Arg

	etcd   *embed.Etcd
	wg     *sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

func SetDefaults(cp *ControlPlain) {
	cp.wg = new(sync.WaitGroup)
	cp.ctx, cp.cancel = context.WithCancel(context.Background())

	setDefaults(cp)
}

func setDefaults(cfg *ControlPlain) {
	if len(cfg.AdvertiseAddress) == 0 {
		cfg.AdvertiseAddress = "0.0.0.0"
	}
	if cfg.BindPort == 0 {
		cfg.BindPort = 6443
	}
	if len(cfg.ClusterName) == 0 {
		cfg.ClusterName = defaultClusterName
	}
	if len(cfg.DNSDomain) == 0 {
		cfg.DNSDomain = defaultDnsDomain
	}
	if len(cfg.EtcdDataDir) == 0 {
		cfg.EtcdDataDir = defaultEtcdDir
	}
	if len(cfg.KubernetesDir) == 0 {
		cfg.KubernetesDir = defaultKubernetesDir
	}
	if len(cfg.CertificatesDir) == 0 {
		cfg.CertificatesDir = filepath.Join(cfg.KubernetesDir, "pki")
	}
	if len(cfg.PodSubnet) == 0 {
		cfg.PodSubnet = defaultPodSubnet
	}
	if len(cfg.ServiceSubnet) == 0 {
		cfg.ServiceSubnet = defaultServicesSubnet
	}
	if len(cfg.ClusterName) == 0 {
		cfg.ClusterName = defaultClusterName
	}
	if len(cfg.EtcdSocketPath) == 0 {
		cfg.EtcdSocketPath = etcdSocketPath
	}
}

func (c *ControlPlain) Etcd() {
	if err := os.MkdirAll(c.EtcdDataDir, 0700); err != nil {
		klog.Fatalf("create etcd (kine) data directory failed: %v", err)
	}

	etcdConfig := embed.NewConfig()
	u, _ := url.Parse("unix://" + c.EtcdSocketPath)
	etcdConfig.ListenClientUrls = []url.URL{*u}
	etcdConfig.Dir = c.EtcdDataDir

	etcd, err := embed.StartEtcd(etcdConfig)
	if err != nil {
		klog.Fatal(err)
	}

	c.etcd = etcd

	<-etcd.Server.ReadyNotify()
}

func (c *ControlPlain) APIServer() {
	cmd := NewAPIServerCommand()
	cmd.SetArgs(apiServerCommand(c))

	c.wg.Add(1)
	go func() {
		if err := cmd.ExecuteContext(c.ctx); err != nil {
			klog.Errorf("kube-apiserver exited: %v", err)
			c.cancel()
		}
		c.wg.Done()
	}()
}

func (c *ControlPlain) ControllerManager() {
	cmd := NewControllerManagerCommand()
	cmd.SetArgs(controllerManagerCommand(c))

	c.wg.Add(1)
	go func() {
		if err := cmd.ExecuteContext(c.ctx); err != nil {
			klog.Errorf("kube-controller-manager exited: %v", err)
			c.cancel()
		}
		c.wg.Done()
	}()
}

func (c *ControlPlain) Scheduler() {
	cmd := NewSchedulerCommand()
	cmd.SetArgs(schedulerCommand(c))

	c.wg.Add(1)
	go func() {
		if err := cmd.ExecuteContext(c.ctx); err != nil {
			klog.Errorf("kube-scheduler exited: %v", err)
			c.cancel()
		}
		c.wg.Done()
	}()
}

func (c *ControlPlain) Shutdown() {
	c.cancel()
	c.wg.Wait()
	c.etcd.Close()
}

// https://github.com/kubernetes/kubernetes/blob/c9024e7ae628f1473a6cac28e7bd6cd8e64f872f/cmd/kubeadm/app/phases/controlplane/manifests.go#L166
// apiServerCommand builds the right API server command from the given config object and version
func apiServerCommand(cfg *ControlPlain) []string {
	defaultArguments := []kubeadmapi.Arg{
		{Name: "advertise-address", Value: cfg.AdvertiseAddress},
		{Name: "cert-dir", Value: cfg.CertificatesDir},
		{Name: "enable-admission-plugins", Value: "NodeRestriction"},
		{Name: "service-cluster-ip-range", Value: cfg.ServiceSubnet},
		{Name: "service-account-key-file", Value: filepath.Join(cfg.CertificatesDir, kubeadmconstants.ServiceAccountPublicKeyName)},
		{Name: "service-account-signing-key-file", Value: filepath.Join(cfg.CertificatesDir, kubeadmconstants.ServiceAccountPrivateKeyName)},
		{Name: "service-account-issuer", Value: fmt.Sprintf("https://kubernetes.default.svc.%s", cfg.DNSDomain)},
		{Name: "client-ca-file", Value: filepath.Join(cfg.CertificatesDir, kubeadmconstants.CACertName)},
		{Name: "tls-cert-file", Value: filepath.Join(cfg.CertificatesDir, kubeadmconstants.APIServerCertName)},
		{Name: "tls-private-key-file", Value: filepath.Join(cfg.CertificatesDir, kubeadmconstants.APIServerKeyName)},
		{Name: "secure-port", Value: fmt.Sprintf("%d", cfg.BindPort)},
		{Name: "allow-privileged", Value: "true"},
		{Name: "requestheader-username-headers", Value: "X-Remote-User"},
		{Name: "requestheader-group-headers", Value: "X-Remote-Group"},
		{Name: "requestheader-extra-headers-prefix", Value: "X-Remote-Extra-"},
		{Name: "requestheader-client-ca-file", Value: filepath.Join(cfg.CertificatesDir, kubeadmconstants.FrontProxyCACertName)},
		{Name: "requestheader-allowed-names", Value: "front-proxy-client"},
		{Name: "proxy-client-cert-file", Value: filepath.Join(cfg.CertificatesDir, kubeadmconstants.FrontProxyClientCertName)},
		{Name: "proxy-client-key-file", Value: filepath.Join(cfg.CertificatesDir, kubeadmconstants.FrontProxyClientKeyName)},
	}

	defaultArguments = kubeadmapi.SetArgValues(defaultArguments, "etcd-servers", "unix://"+cfg.EtcdSocketPath, 1)

	if cfg.APIServerExtraArgs == nil {
		cfg.APIServerExtraArgs = []kubeadmapi.Arg{}
	}
	authzVal, _ := kubeadmapi.GetArgValue(cfg.APIServerExtraArgs, "authorization-mode", -1)
	_, hasStructuredAuthzVal := kubeadmapi.GetArgValue(cfg.APIServerExtraArgs, "authorization-config", -1)
	if hasStructuredAuthzVal == -1 {
		defaultArguments = kubeadmapi.SetArgValues(defaultArguments, "authorization-mode", getAuthzModes(authzVal), 1)
	}

	return kubeadmutil.ArgumentsToCommand(defaultArguments, cfg.APIServerExtraArgs)
}

// getAuthzModes gets the authorization-related parameters to the api server
// Node,RBAC is the default mode if nothing is passed to kubeadm. User provided modes override
// the default.
func getAuthzModes(authzModeExtraArgs string) string {
	defaultMode := []string{
		kubeadmconstants.ModeNode,
		kubeadmconstants.ModeRBAC,
	}

	if len(authzModeExtraArgs) > 0 {
		mode := make([]string, 0)
		for _, requested := range strings.Split(authzModeExtraArgs, ",") {
			if isValidAuthzMode(requested) {
				mode = append(mode, requested)
			} else {
				klog.Warningf("ignoring unknown kube-apiserver authorization-mode %q", requested)
			}
		}

		// only return the user provided mode if at least one was valid
		if len(mode) > 0 {
			if !compareAuthzModes(defaultMode, mode) {
				klog.Warningf("the default kube-apiserver authorization-mode is %q; using %q",
					strings.Join(defaultMode, ","),
					strings.Join(mode, ","),
				)
			}
			return strings.Join(mode, ",")
		}
	}
	return strings.Join(defaultMode, ",")
}

// compareAuthzModes compares two given authz modes and returns false if they do not match
func compareAuthzModes(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, m := range a {
		if m != b[i] {
			return false
		}
	}
	return true
}

func isValidAuthzMode(authzMode string) bool {
	allModes := []string{
		kubeadmconstants.ModeNode,
		kubeadmconstants.ModeRBAC,
		kubeadmconstants.ModeWebhook,
		kubeadmconstants.ModeABAC,
		kubeadmconstants.ModeAlwaysAllow,
		kubeadmconstants.ModeAlwaysDeny,
	}

	for _, mode := range allModes {
		if authzMode == mode {
			return true
		}
	}
	return false
}

// controllerManagerCommand builds the right controller manager command from the given config object and version
func controllerManagerCommand(cfg *ControlPlain) []string {

	kubeconfigFile := filepath.Join(cfg.KubernetesDir, kubeadmconstants.ControllerManagerKubeConfigFileName)
	caFile := filepath.Join(cfg.CertificatesDir, kubeadmconstants.CACertName)

	defaultArguments := []kubeadmapi.Arg{
		{Name: "bind-address", Value: "127.0.0.1"},
		{Name: "cert-dir", Value: cfg.CertificatesDir},
		{Name: "leader-elect", Value: "false"},
		{Name: "kubeconfig", Value: kubeconfigFile},
		{Name: "authentication-kubeconfig", Value: kubeconfigFile},
		{Name: "authorization-kubeconfig", Value: kubeconfigFile},
		{Name: "client-ca-file", Value: caFile},
		{Name: "requestheader-client-ca-file", Value: filepath.Join(cfg.CertificatesDir, kubeadmconstants.FrontProxyCACertName)},
		{Name: "root-ca-file", Value: caFile},
		{Name: "service-account-private-key-file", Value: filepath.Join(cfg.CertificatesDir, kubeadmconstants.ServiceAccountPrivateKeyName)},
		{Name: "cluster-signing-cert-file", Value: caFile},
		{Name: "cluster-signing-key-file", Value: filepath.Join(cfg.CertificatesDir, kubeadmconstants.CAKeyName)},
		{Name: "use-service-account-credentials", Value: "true"},
		{Name: "controllers", Value: "*,bootstrapsigner,tokencleaner"},
	}

	// Let the controller-manager allocate Node CIDRs for the Pod network.
	// Each node will get a subspace of the address CIDR provided with --pod-network-cidr.
	if cfg.PodSubnet != "" {
		defaultArguments = kubeadmapi.SetArgValues(defaultArguments, "allocate-node-cidrs", "true", 1)
		defaultArguments = kubeadmapi.SetArgValues(defaultArguments, "cluster-cidr", cfg.PodSubnet, 1)
		if cfg.ServiceSubnet != "" {
			defaultArguments = kubeadmapi.SetArgValues(defaultArguments, "service-cluster-ip-range", cfg.ServiceSubnet, 1)
		}
	}

	// Set cluster name
	if cfg.ClusterName != "" {
		defaultArguments = kubeadmapi.SetArgValues(defaultArguments, "cluster-name", cfg.ClusterName, 1)
	}

	return kubeadmutil.ArgumentsToCommand(defaultArguments, cfg.ControllerManagerExtraArgs)
}

// schedulerCommand builds the right scheduler command from the given config object and version
func schedulerCommand(cfg *ControlPlain) []string {
	kubeconfigFile := filepath.Join(cfg.KubernetesDir, kubeadmconstants.SchedulerKubeConfigFileName)
	defaultArguments := []kubeadmapi.Arg{
		{Name: "bind-address", Value: "127.0.0.1"},
		{Name: "cert-dir", Value: cfg.CertificatesDir},
		{Name: "leader-elect", Value: "false"},
		{Name: "kubeconfig", Value: kubeconfigFile},
		{Name: "authentication-kubeconfig", Value: kubeconfigFile},
		{Name: "authorization-kubeconfig", Value: kubeconfigFile},
	}

	return kubeadmutil.ArgumentsToCommand(defaultArguments, cfg.SchedulerExtraArgs)
}
