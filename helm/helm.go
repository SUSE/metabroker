/*
Copyright 2020 SUSE

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

package helm

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
)

const (
	backendStorageDriver = "secret"
	defaultNamespace     = "default"

	// Empty values for these mean the internal defaults will be used.
	defaultKubeConfig = ""
	defaultContext    = ""

	// As old versions of Kubernetes had a limit on names of 63 characters, Helm uses 53, reserving
	// 10 characters for charts to add data.
	helmMaxNameLength = 53
)

// Client is the interface that wraps all the Helm methods.
type Client interface {
	Install(name string, chartInfo ChartInfo, opts InstallOpts) (*release.Release, error)
	Uninstall(name string, opts UninstallOpts) error
	Get(name string, opts GetOpts) (*release.Release, error)
	ListResources(rel *release.Release) (kube.ResourceList, error)
	IsReady(rel *release.Release) (bool, error)
}

// ChartInfo contains information necessary to identify a helm chart archive for installation.
type ChartInfo struct {
	URL, SHA256Sum string
}

// NamespaceOpt represents a namespace string that implements the ValueOrDefault
// method. It's used to return the default namespace in the case of the value
// being empty.
type NamespaceOpt string

// ValueOrDefault returns the default namespace if the NamespaceOpt value is not set.
func (ns *NamespaceOpt) ValueOrDefault() string {
	if *ns == "" {
		return defaultNamespace
	}
	return string(*ns)
}

// InstallOpts are the Helm install options.
type InstallOpts struct {
	Atomic      bool
	Description string
	Namespace   NamespaceOpt
	Timeout     time.Duration
	Values      map[string]interface{}
	Wait        bool
}

// UninstallOpts are the Helm uninstall options.
type UninstallOpts struct {
	Description string
	Namespace   NamespaceOpt
	Timeout     time.Duration
}

// GetOpts are the Helm get options.
type GetOpts struct {
	Namespace NamespaceOpt
}

type client struct {
	chartCache    *ChartCache
	readyReleases map[namespacedName](chan error)
}

// NewClient constructs a new Client.
func NewClient(chartCache *ChartCache) Client {
	return &client{
		chartCache:    chartCache,
		readyReleases: make(map[namespacedName](chan error)),
	}
}

// Install installs a Helm chart from a ChartInfo using its URL and SHA 256 sum to verify its
// integrity. The chart tarball is cached for future uses based on the checksum.
func (c *client) Install(name string, chartInfo ChartInfo, opts InstallOpts) (*release.Release, error) {
	if len(name) > helmMaxNameLength {
		err := fmt.Errorf(
			"invalid release name %q: names cannot exceed %d characters",
			name,
			helmMaxNameLength)
		return nil, fmt.Errorf("failed to install Helm chart %q: %w", name, err)
	}

	chartFile, err := c.chartCache.Fetch(chartInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to install Helm chart %q: %w", name, err)
	}
	defer chartFile.Close()

	chart, err := loader.LoadArchive(chartFile)
	if err != nil {
		return nil, fmt.Errorf("failed to install Helm chart %q: %w", name, err)
	}

	namespace := opts.Namespace.ValueOrDefault()
	cfg, err := c.config(namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to install Helm chart %q: %w", name, err)
	}
	client := action.NewInstall(cfg)
	client.ReleaseName = name
	client.Namespace = namespace
	client.Atomic = opts.Atomic
	client.Description = opts.Description
	client.Timeout = opts.Timeout
	client.Wait = opts.Wait

	rel, err := client.Run(chart, opts.Values)
	if err != nil {
		return nil, fmt.Errorf("failed to install Helm chart %q: %w", name, err)
	}

	return rel, nil
}

// Uninstall uninstalls a Helm release by name. Uninstall returns nil if the release doesn't exist.
func (c *client) Uninstall(name string, opts UninstallOpts) error {
	namespace := opts.Namespace.ValueOrDefault()
	cfg, err := c.config(namespace)
	if err != nil {
		return fmt.Errorf("failed to uninstall Helm release %q: %w", name, err)
	}
	client := action.NewUninstall(cfg)
	client.Description = opts.Description
	if _, err := client.Run(name); err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			return nil
		}
		return fmt.Errorf("failed to uninstall Helm release %q: %w", name, err)
	}
	return nil
}

// Get gets a Helm installation using its name.
func (c *client) Get(name string, opts GetOpts) (*release.Release, error) {
	namespace := opts.Namespace.ValueOrDefault()
	cfg, err := c.config(namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get Helm release %q: %w", name, err)
	}
	client := action.NewGet(cfg)
	rel, err := client.Run(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get Helm release %q: %w", name, err)
	}
	return rel, nil
}

// ListResources lists the desired state of the Kubernetes resources for a given Helm release.
func (c *client) ListResources(rel *release.Release) (kube.ResourceList, error) {
	cfg, err := c.config(rel.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources for Helm release %q: %w", rel.Name, err)
	}
	current, err := cfg.KubeClient.Build(bytes.NewBufferString(rel.Manifest), false)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources for Helm release %q: %w", rel.Name, err)
	}
	return current, nil
}

// IsReady determines whether a Helm installation is ready or not. This method blocks for up to 5
// seconds when the background helm.Wait blocking method is not running yet, giving it a chance to
// return on the first call whether the release is ready or not instead of just returning false.
func (c *client) IsReady(rel *release.Release) (bool, error) {
	nsName := namespacedName{
		name:      rel.Name,
		namespace: rel.Namespace,
	}
	ch, ok := c.readyReleases[nsName]
	if ok {
		select {
		case err := <-ch:
			close(ch)
			delete(c.readyReleases, nsName)
			if err != nil {
				return false, fmt.Errorf("failed to determine whether the Helm release %q is ready or not: %w", rel.Name, err)
			}
			return true, nil
		default:
			return false, nil
		}
	}

	objs, err := c.ListResources(rel)
	if err != nil {
		return false, fmt.Errorf("failed to determine whether the Helm release %q is ready or not: %w", rel.Name, err)
	}

	cfg, err := c.config(rel.Namespace)
	if err != nil {
		return false, fmt.Errorf("failed to determine whether the Helm release %q is ready or not: %w", rel.Name, err)
	}

	ch = make(chan error, 1)
	c.readyReleases[nsName] = ch

	go func() { ch <- cfg.KubeClient.Wait(objs, 5*time.Minute) }()

	select {
	case err := <-ch:
		close(ch)
		delete(c.readyReleases, nsName)
		if err != nil {
			return false, fmt.Errorf("failed to determine whether the Helm release %q is ready or not: %w", rel.Name, err)
		}
		return true, nil
	// If the Wait method was just invoked, we give a chance for it to complete in less than 5
	// seconds for the first time instead of returning not ready right away. This is useful for
	// reconciliation loops that need to frequently check whether the Helm release is ready or not.
	case <-time.After(5 * time.Second):
		return false, nil
	}
}

func (c *client) config(namespace string) (*action.Configuration, error) {
	restGetter := kube.GetConfig(defaultKubeConfig, defaultContext, namespace)
	debug := func(format string, v ...interface{}) {
		// TODO(f0rmiga): provide a logic for Helm debugging logs.
	}
	cfg := &action.Configuration{}
	if err := cfg.Init(restGetter, namespace, backendStorageDriver, debug); err != nil {
		return nil, fmt.Errorf("failed to provide action configuration: %w", err)
	}
	return cfg, nil
}

// ChartCache manages the local chart cache.
type ChartCache struct {
	cachePath string

	osStat     func(name string) (os.FileInfo, error)
	osOpenFile func(name string, flag int, perm os.FileMode) (*os.File, error)
	httpGetter HTTPGetter

	mutex sync.Mutex
}

// NewChartCache constructs a new ChartCache.
func NewChartCache(cachePath string) *ChartCache {
	return &ChartCache{
		cachePath:  cachePath,
		osStat:     os.Stat,
		osOpenFile: os.OpenFile,
		httpGetter: http.DefaultClient,
	}
}

// Fetch downloads a chart tarball if needed, adding it to the cache. The
// returned tarball is always a handle to the cached file.
func (cc *ChartCache) Fetch(chartInfo ChartInfo) (io.ReadCloser, error) {
	// TODO: a better strategy to handling the cache is to build it when a Plan is reconciled.
	// Instances should then wait for the Plan to become ready, i.e. waiting for the chart to be
	// cached.
	cc.mutex.Lock()
	defer cc.mutex.Unlock()

	fileName := fmt.Sprintf("%s.tgz", chartInfo.SHA256Sum)
	filePath := path.Join(cc.cachePath, fileName)
	if _, err := cc.osStat(filePath); err == nil {
		r, err := cc.osOpenFile(filePath, os.O_RDONLY, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch chart from %q: %w", chartInfo.URL, err)
		}
		return r, nil
	} else if !os.IsNotExist(err) {
		// The error exists and it's not of the type non-existent file. I.e. an
		// unexpected error.
		return nil, fmt.Errorf("failed to fetch chart from %q: %w", chartInfo.URL, err)
	}

	res, err := cc.httpGetter.Get(chartInfo.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chart from %q: %w", chartInfo.URL, err)
	}
	defer res.Body.Close()
	if err := cc.cache(filePath, res.Body, chartInfo.SHA256Sum); err != nil {
		return nil, fmt.Errorf("failed to fetch chart from %q: %w", chartInfo.URL, err)
	}

	r, err := cc.osOpenFile(filePath, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chart from %q: %w", chartInfo.URL, err)
	}
	return r, nil
}

func (cc *ChartCache) cache(filePath string, data io.Reader, sha256sum string) error {
	w, err := cc.osOpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to cache %q: %w", filePath, err)
	}
	defer w.Close()

	h := sha256.New()

	if _, err := fanout(data, w, h); err != nil {
		w.Close()
		os.Remove(filePath)
		return fmt.Errorf("failed to cache %q: %w", filePath, err)
	}

	calculatedHashSum := fmt.Sprintf("%x", h.Sum(nil))
	if sha256sum != calculatedHashSum {
		w.Close()
		os.Remove(filePath)
		err := fmt.Errorf(
			"provided checksum %q does not match calculated %q",
			sha256sum,
			calculatedHashSum)
		return fmt.Errorf("failed to cache %q: %w", filePath, err)
	}

	return nil
}

// HTTPGetter wraps the HTTP Get method.
type HTTPGetter interface {
	Get(url string) (resp *http.Response, err error)
}

// fanout writes the data from the reader to the many writers it can take.
func fanout(r io.Reader, ws ...io.Writer) (written int64, err error) {
	var ir io.Reader = r
	for _, w := range ws {
		ir = io.TeeReader(ir, w)
	}
	return io.Copy(ioutil.Discard, ir)
}

// AsVersioned wraps "helm.sh/helm/v3/pkg/kube".AsVersioned for easier reuse. It converts the given
// info into a runtime.Object with the correct group and version set.
func AsVersioned(info *resource.Info) runtime.Object {
	return kube.AsVersioned(info)
}

type namespacedName struct {
	name, namespace string
}
