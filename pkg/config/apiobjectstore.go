/*
   Copyright 2021 The Kubermatic Kubernetes Platform contributors.

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

package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/kubermatic-labs/registryman/pkg/acr"
	api "github.com/kubermatic-labs/registryman/pkg/apis/registryman.kubermatic.com/v1alpha1"
	"github.com/kubermatic-labs/registryman/pkg/config/registry"
	_ "github.com/kubermatic-labs/registryman/pkg/harbor"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/kube-openapi/pkg/validation/validate"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
	}
	err := api.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}
	scheme.AddKnownTypeWithName(secret.GroupVersionKind(), secret)
}

// ApiObjectStore is the database of the configured resources (Projects,
// Registries and Scanners).
type ApiObjectStore struct {
	store      map[schema.GroupVersionKind][]runtime.Object
	serializer *json.Serializer
	path       string
}

// WriteManifest serializes the object specified by the obj parameter. The
// filename parameter specifies the name of the file to be created. The path
// where the file is created is set when the ReadManifests function
// creates the ApiObjectStore.
func (aos *ApiObjectStore) WriteManifest(filename string, obj runtime.Object) error {
	fName := filepath.Join(aos.path, filename)
	f, err := os.Create(fName)
	if err != nil {
		return err
	}
	defer f.Close()

	err = aos.serializer.Encode(obj, f)
	if err != nil {
		return err
	}
	return nil
}

// RemoveManifest removes the file from the filesystem. The path where the file
// is removed from is set when the ReadManifests function creates the
// ApiObjectStore.
func (aos *ApiObjectStore) RemoveManifest(filename string) error {
	fName := filepath.Join(aos.path, filename)
	return os.Remove(fName)
}

// ReadManifests creates a new ApiObjectStore. It reads all files under path.
// The files are deserialized and validated.
func ReadManifests(path string) (*ApiObjectStore, error) {
	aos := &ApiObjectStore{
		path: path,
	}
	aos.serializer = json.NewSerializerWithOptions(
		json.DefaultMetaFactory,
		scheme,
		scheme,
		json.SerializerOptions{
			Yaml:   true,
			Pretty: true,
			Strict: true,
		})
	dir, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(16 * 1024)
	if err != nil {
		return nil, err
	}
	aos.store = make(map[schema.GroupVersionKind][]runtime.Object)
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".yaml") {
			// skip the non-yaml files
			continue
		}
		f, err := os.Open(filepath.Join(path, entry.Name()))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		b, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		o, gvk, err := aos.serializer.Decode(b, nil, nil)
		if err != nil {
			// This is not a valid Kubernetes resource. Let's skip it.
			logger.V(-1).Info("file is not a valid resource",
				"error", err.Error(),
				"filename", entry.Name(),
			)
			continue
		}
		err = validateObjects(o, gvk)
		if err != nil {
			return nil, fmt.Errorf("validation error during inspecting %s:\n %w", entry.Name(), err)
		}
		objects, found := aos.store[*gvk]
		if found {
			aos.store[*gvk] = append(objects, o)
		} else {
			aos.store[*gvk] = []runtime.Object{o}
		}
	}
	err = aos.validate()
	if err != nil {
		return nil, err
	}
	return aos, nil
}

// checkGlobalRegistryCount checks that there is 1 or 0 registry configured with
// the type GlobalHub.
func checkGlobalRegistryCount(registries []*api.Registry) error {
	globalRegistries := make([]string, 0)
	for _, registry := range registries {
		if registry.Spec.Role == "GlobalHub" {
			globalRegistries = append(globalRegistries, registry.Name)
		}
	}
	if len(globalRegistries) >= 2 {
		for _, registry := range globalRegistries {
			logger.V(-2).Info("Multiple Global Registries found",
				"registry_name", registry)
		}
		return ErrValidationMultipleGlobalRegistries
	}
	return nil
}

// checkLocalRegistryNamesInProjects checks that the registries referenced by
// the local projects exist.
func checkLocalRegistryNamesInProjects(registries []*api.Registry, projects []*api.Project) error {
	localRegistries := make([]string, 0)
	for _, registry := range registries {
		if registry.Spec.Role == "Local" {
			localRegistries = append(localRegistries, registry.Name)
		}
	}
	invalidProjects := make([]string, 0)
	for _, project := range projects {
		if project.Spec.Type == api.LocalProjectType {
			localRegistryExists := false
			for _, localRegistry := range project.Spec.LocalRegistries {
				for _, registry := range localRegistries {
					if localRegistry == registry {
						localRegistryExists = true
						break
					}
				}
				if !localRegistryExists {
					invalidProjects = append(invalidProjects, project.Name)
					logger.V(-2).Info("Local registry not exists",
						"project_name", project.Name,
						"registry_name", localRegistry)
				}
				localRegistryExists = false
			}
		}
	}

	if len(invalidProjects) > 0 {
		return ErrValidationInvalidLocalRegistryInProject
	}
	return nil
}

// checkScannerNamesInProjects checks that the scanners referenced by the
// projects exist.
func checkScannerNamesInProjects(projects []*api.Project, scanners []*api.Scanner) error {
	isProjectInvalid := false
	scannerNames := map[string]*api.Scanner{}
	for _, scanner := range scanners {
		scannerNames[scanner.GetName()] = scanner
	}
	for _, project := range projects {
		if project.Spec.Scanner != "" &&
			scannerNames[project.Spec.Scanner] == nil {
			// there is a project with invalid scanner name
			logger.V(-2).Info("Project refers to non-existing scanner",
				"project_name", project.Name,
				"scanner_name", project.Spec.Scanner)
			isProjectInvalid = true
		}
	}

	if isProjectInvalid {
		return ErrValidationScannerNameReference
	}
	return nil
}

// checkScannerNameUniqueness checks that there are no 2 scanners with the same
// name.
func checkScannerNameUniqueness(scanners []*api.Scanner) error {
	scannerNames := map[string]*api.Scanner{}
	for _, scanner := range scanners {
		scannerName := scanner.GetName()
		if scannerNames[scannerName] != nil {
			logger.V(-2).Info("Multiple scanners configured with the same name",
				"scanner_name", scannerName,
			)
			return ErrValidationScannerNameNotUnique
		}
		scannerNames[scannerName] = scanner
	}
	return nil
}

// checkProjectNameUniqueness checks that there are no 2 projects with the same
// name.
func checkProjectNameUniqueness(projects []*api.Project) error {
	projectNames := map[string]*api.Project{}
	for _, project := range projects {
		projectName := project.GetName()
		if projectNames[projectName] != nil {
			logger.V(-2).Info("Multiple projects configured with the same name",
				"project_name", projectName,
			)
			return ErrValidationProjectNameNotUnique
		}
		projectNames[projectName] = project
	}
	return nil
}

// checkRegistryNameUniqueness checks that there are no 2 registries with the
// same name.
func checkRegistryNameUniqueness(registries []*api.Registry) error {
	registryNames := map[string]*api.Registry{}
	for _, registry := range registries {
		registryName := registry.GetName()
		if registryNames[registryName] != nil {
			logger.V(-2).Info("Multiple registries configured with the same name",
				"registry_name", registryName,
			)
			return ErrValidationRegistryNameNotUnique
		}
		registryNames[registryName] = registry
	}
	return nil
}

// validate performs all validations that require the full context, i.e. all
// resources parsed.
func (aos *ApiObjectStore) validate() error {
	registries := aos.GetRegistries()
	projects := aos.GetProjects()
	scanners := aos.GetScanners()

	// Forcing maximum one Global registry
	err := checkGlobalRegistryCount(registries)
	if err != nil {
		return err
	}

	// Checking local registry names in all local projects
	err = checkLocalRegistryNamesInProjects(registries, projects)
	if err != nil {
		return err
	}

	// Checking scanner names in all projects
	err = checkScannerNamesInProjects(projects, scanners)
	if err != nil {
		return err
	}

	// Checking scanner name uniqueness
	err = checkScannerNameUniqueness(scanners)
	if err != nil {
		return err
	}

	// Checking project name uniqueness
	err = checkProjectNameUniqueness(projects)
	if err != nil {
		return err
	}

	// Checking registry name uniqueness
	err = checkRegistryNameUniqueness(registries)
	if err != nil {
		return err
	}
	return nil
}

// checkProject checks the validation rules of the project resources. This
// function contains the checks that can be performed on a single Project
// resource.
func checkProject(project *api.Project) error {
	for _, member := range project.Spec.Members {
		if member.Type == api.GroupMemberType && member.DN == "" {
			return ErrValidationGroupWithoutDN
		}
	}
	return nil
}

// validateObjects perform the CRD level validation on each object.
func validateObjects(o runtime.Object, gvk *schema.GroupVersionKind) error {
	var results *validate.Result

	switch gvk.Kind {
	case "Registry":
		results = api.RegistryValidator.Validate(o)
	case "Project":
		results = api.ProjectValidator.Validate(o)
	case "Scanner":
		results = api.ScannerValidator.Validate(o)
	default:
		// We have parsed a Kubernetes resource which is not our kind.
		// Don't validate it.
		return nil
	}

	if results.HasErrors() {
		return results.AsError()
	}
	switch gvk.Kind {
	case "Project":
		return checkProject(o.(*api.Project))
	default:
		return nil
	}
}

// GetRegistries returns the parsed registries as API objects.
func (aos *ApiObjectStore) GetRegistries() []*api.Registry {
	registryObjects, found := aos.store[api.SchemeGroupVersion.WithKind("Registry")]
	if !found {
		return []*api.Registry{}
	}
	registries := make([]*api.Registry, len(registryObjects))
	for i, reg := range registryObjects {
		registries[i] = reg.(*api.Registry)
	}
	return registries
}

// GetProjects returns the parsed projects as API objects.
func (aos *ApiObjectStore) GetProjects() []*api.Project {
	projectObjects, found := aos.store[api.SchemeGroupVersion.WithKind("Project")]
	if !found {
		return []*api.Project{}
	}
	projects := make([]*api.Project, len(projectObjects))
	for i, reg := range projectObjects {
		projects[i] = reg.(*api.Project)
	}
	return projects
}

// GetScanners returns the parsed scanners as API objects.
func (aos *ApiObjectStore) GetScanners() []*api.Scanner {
	scannerObjects, found := aos.store[api.SchemeGroupVersion.WithKind("Scanner")]
	if !found {
		return []*api.Scanner{}
	}
	scanners := make([]*api.Scanner, len(scannerObjects))
	for i, reg := range scannerObjects {
		scanners[i] = reg.(*api.Scanner)
	}
	return scanners
}

// ExpectedProvider is a database of the resources which implement the
// interfaces defines in the globalregistry package.
//
// The resources in the database usually show the expected state of the
// resources.
type ExpectedProvider ApiObjectStore

// ExpectedProvider method turns an ApiObjectStore into an ExpectedProvider.
func (aos *ApiObjectStore) ExpectedProvider() *ExpectedProvider {
	return (*ExpectedProvider)(aos)
}

// GetRegistries returns the Registries of the resource database.
func (expp *ExpectedProvider) GetRegistries() []*registry.Registry {
	registryObjects, found := expp.store[api.SchemeGroupVersion.WithKind("Registry")]
	if !found {
		panic("cannot get registry objects")
	}
	registries := make([]*registry.Registry, len(registryObjects))
	for i, reg := range registryObjects {
		apiRegistry, ok := reg.(*api.Registry)
		if !ok {
			panic("cannot assert registry object")
		}
		registries[i] = registry.New(apiRegistry, (*ApiObjectStore)(expp))
	}
	return registries
}

// GetRegistryByName returns a Registry with the given name from the database.
// If no Registry if found with the specified name, nil is returned.
func (expp *ExpectedProvider) GetRegistryByName(name string) *registry.Registry {
	registryObjects, found := expp.store[api.SchemeGroupVersion.WithKind("Registry")]
	if !found {
		panic("cannot get registry objects")
	}
	for _, reg := range registryObjects {
		apiRegistry, ok := reg.(*api.Registry)
		if !ok {
			panic("cannot assert registry object")
		}
		if apiRegistry.GetName() == name {
			return registry.New(apiRegistry, (*ApiObjectStore)(expp))
		}
	}
	return nil
}
