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

package main

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ks-tool/ks/pkg/k8s"

	"k8s.io/klog/v2"
)

func main() {
	kube := &k8s.ControlPlain{
		KubernetesDir:  "_tmp/etc/kubernetes",
		EtcdDataDir:    "_tmp/example",
		EtcdSocketPath: "_tmp/example/etcd.sock",
	}
	k8s.SetDefaults(kube)

	kube.Etcd()
	kube.APIServer()
	kube.ControllerManager()
	for {
		if httpCheck("https://127.0.0.1:10257/healthz") {
			break
		}

		time.Sleep(time.Second)
	}
	kube.Scheduler()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	kube.Shutdown()
}

func httpCheck(u string) bool {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: -1,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		klog.Errorf("http check fail: %v", err)
		return false
	}

	return resp.StatusCode == 200
}
