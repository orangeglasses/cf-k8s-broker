package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strconv"
	"text/template"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type k8sclient struct {
	dynClient           dynamic.Interface
	restMapper          *restmapper.DeferredDiscoveryRESTMapper
	clientSet           *kubernetes.Clientset
	templateFiles       []string
	templates           *template.Template
	bindTemplate        *template.Template
	getInstanceTemplate *template.Template
}

type masterPortIpProtocol struct {
	IP       string
	Port     int32
	Protocol string
}

func NewK8sClient(kubeconfig, templPath string) *k8sclient {
	//load kubeconfig and create clients
	if kubeconfig == "" {
		kubeconfig = filepath.Join(homedir.HomeDir(), ".kube", "config")
	}
	konfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("unable to load kubeconfig: %s", err.Error())
	}

	cs, err := kubernetes.NewForConfig(konfig)
	if err != nil {
		log.Fatal(err.Error())
	}

	dynClient, err := dynamic.NewForConfig(konfig)
	if err != nil {
		log.Fatal(err.Error())
	}

	discClient, err := discovery.NewDiscoveryClientForConfig(konfig)
	if err != nil {
		log.Fatal(err)
	}

	klient := k8sclient{
		dynClient:  dynClient,
		restMapper: restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discClient)),
		clientSet:  cs,
	}

	//////   Load yaml templates    //////
	glob := templPath + "/*.yml"
	funcMap := template.FuncMap{
		"GetObjectByLabel": klient.GetObjectByLabel,
		"GetObjectByName":  klient.GetObjectByName,
		"base64decode":     base64decode,
	}

	templFiles, err := filepath.Glob(glob)
	if err != nil {
		log.Fatal(err)
	}
	//Sort the filenames. This will apply the files in order. Mainly meant to creat namespace before everything else. We just call the namespace tempalte 00namespace.yml :)
	sort.Strings(templFiles)

	if len(templFiles) == 0 {
		log.Fatalf("No template files found in path: %s", templPath)
	}

	templates, err := template.New("").Funcs(funcMap).ParseGlob(glob)
	if err != nil {
		log.Fatal(err)
	}

	klient.templateFiles = templFiles
	klient.templates = templates
	///////////////////////////

	////// Load bind template ///////
	bindTemplate, err := template.New("bindTemplate.json").Funcs(funcMap).ParseFiles("bindTemplate.json")
	if err != nil {
		log.Fatal(err)
	}
	klient.bindTemplate = bindTemplate
	//////////////////////////////////

	////// Load getInstance template ///////
	getInstanceTemplate, err := template.New("getInstanceTemplate.json").Funcs(funcMap).ParseFiles("getInstanceTemplate.json")
	if err != nil {
		log.Println("Unable to load getInstanceTemplate.json", err)
		klient.getInstanceTemplate = nil
	} else {
		klient.getInstanceTemplate = getInstanceTemplate
	}
	//////////////////////////////////

	return &klient
}

func (k *k8sclient) RenderTemplatesForPlan(ctx context.Context, plan Plan, orgID, spaceID, instanceID string) ([]string, error) {
	data := struct {
		Plan       Plan
		InstanceID string
		OrgID      string
		SpaceID    string
		NodeIPs    []string
		Masters    []masterPortIpProtocol
	}{
		Plan:       plan,
		InstanceID: instanceID,
		OrgID:      orgID,
		SpaceID:    spaceID,
		NodeIPs:    k.GetNodeIPs(ctx),
		Masters:    k.GetMasterIPs(ctx),
	}

	var output []string

	for _, file := range k.templateFiles {
		rendered := new(bytes.Buffer)
		err := k.templates.ExecuteTemplate(rendered, filepath.Base(file), data)
		if err != nil {
			return nil, err
		}

		if len(rendered.String()) > 15 { //less than 15 characters can never be a valid k8s yaml so don't include it.
			output = append(output, rendered.String())
		}
	}

	return output, nil
}

func (k *k8sclient) RenderJsonTemplate(ctx context.Context, templ *template.Template, instanceID string) (*bytes.Buffer, error) {
	data := struct {
		InstanceID string
	}{
		InstanceID: instanceID,
	}

	rendered := new(bytes.Buffer)

	err := templ.Execute(rendered, data)
	if err != nil {
		return nil, err
	}

	return rendered, nil
}

func (k *k8sclient) RenderBindTemplate(ctx context.Context, instanceID string) (map[string]string, error) {
	rendered, err := k.RenderJsonTemplate(ctx, k.bindTemplate, instanceID)
	if err != nil {
		return nil, err
	}

	var creds map[string]string
	err = json.Unmarshal(rendered.Bytes(), &creds)
	if err != nil {
		return nil, err
	}

	return creds, nil

}

func (k *k8sclient) RenderGetInstanceTemplate(ctx context.Context, instanceID string) (map[string]interface{}, error) {
	rendered, err := k.RenderJsonTemplate(ctx, k.getInstanceTemplate, instanceID)
	if err != nil {
		return nil, err
	}

	var instanceDetails map[string]interface{}
	err = json.Unmarshal(rendered.Bytes(), &instanceDetails)
	if err != nil {
		return nil, err
	}

	return instanceDetails, nil

}

func (k *k8sclient) GetNodeIPs(ctx context.Context) []string {
	var nodeIPs []string
	nodeList, err := k.clientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Error listing nodes: %s", err)
	}
	for _, node := range nodeList.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == "InternalIP" {
				nodeIPs = append(nodeIPs, addr.Address)
			}
		}
	}

	return nodeIPs
}

func (k *k8sclient) GetMasterIPs(ctx context.Context) []masterPortIpProtocol {
	var masterIPs []masterPortIpProtocol
	masterEndpoints, err := k.clientSet.CoreV1().Endpoints("default").Get(ctx, "kubernetes", metav1.GetOptions{})
	if err != nil {
		log.Printf("Error getting master service: %s", err)
	}

	for _, s := range masterEndpoints.Subsets {
		var master masterPortIpProtocol
		for _, ip := range s.Addresses {
			master.IP = ip.IP
			for _, port := range s.Ports {
				master.Port = port.Port
				master.Protocol = fmt.Sprintf("%s", port.Protocol)
			}
		}
		masterIPs = append(masterIPs, master)
	}

	return masterIPs
}

func (k *k8sclient) decodeAndEnrichYaml(yamlIn, instanceID string) (*meta.RESTMapping, *unstructured.Unstructured, error) {
	obj := unstructured.Unstructured{}

	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, gvk, err := dec.Decode([]byte(yamlIn), nil, &obj)
	if err != nil {
		log.Fatal(err)
	}

	mapping, err := k.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, nil, err
	}

	//Set the instanceID label. We need this later for binding
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["instanceID"] = instanceID
	obj.SetLabels(labels)

	return mapping, &obj, nil
}

func (k *k8sclient) IsNsSet(obj *unstructured.Unstructured) bool {
	if obj.GetNamespace() == "" {
		if obj.GetKind() != "Namespace" {
			return false
		}
	}

	return true
}

func (k *k8sclient) CreateFromYaml(ctx context.Context, yamlIn, instanceID string) error {
	mapping, obj, err := k.decodeAndEnrichYaml(yamlIn, instanceID)
	if err != nil {
		return err
	}

	if !k.IsNsSet(obj) {
		return fmt.Errorf("Error validating object %s/%s, %s. Skipping object creation", obj.GetKind(), obj.GetName(), err)
	}

	_, err = k.dynClient.Resource(mapping.Resource).Namespace(obj.GetNamespace()).Create(ctx, obj, metav1.CreateOptions{})

	return err
}

func (k *k8sclient) UpdateFromYaml(ctx context.Context, yamlIn, instanceID string) error {
	mapping, obj, err := k.decodeAndEnrichYaml(yamlIn, instanceID)
	if err != nil {
		return err
	}

	if !k.IsNsSet(obj) {
		return fmt.Errorf("Error validating object %s/%s, %s. Skipping object update", obj.GetKind(), obj.GetName(), err)
	}

	currentObj, err := k.dynClient.Resource(mapping.Resource).Namespace(obj.GetNamespace()).Get(ctx, obj.GetName(), metav1.GetOptions{})
	if err != nil {
		return err
	}

	currentLabels := currentObj.GetLabels()
	newLabels := obj.GetLabels()
	for key, label := range newLabels {
		currentLabels[key] = label
	}
	obj.SetLabels(currentLabels)

	obj.SetResourceVersion(currentObj.GetResourceVersion())

	_, err = k.dynClient.Resource(mapping.Resource).Namespace(obj.GetNamespace()).Update(ctx, obj, metav1.UpdateOptions{})

	return err
}

func (k *k8sclient) GetObject(ctx context.Context, instanceID, yamlIn string) (*unstructured.Unstructured, error) {
	mapping, obj, err := k.decodeAndEnrichYaml(yamlIn, instanceID)
	if err != nil {
		return nil, err
	}

	robj, err := k.dynClient.Resource(mapping.Resource).Namespace(obj.GetNamespace()).Get(ctx, obj.GetName(), metav1.GetOptions{})

	return robj, err
}

func (k *k8sclient) GetObjectByLabel(namespace, kind, label, labelValue string) *unstructured.Unstructured {
	mapping, err := k.restMapper.RESTMapping(schema.ParseGroupKind(kind))
	if err != nil {
		log.Printf("Error getting restmapping for %s: %s", kind, err)
		return nil
	}

	var list *unstructured.UnstructuredList

	if namespace != "" {
		list, err = k.dynClient.Resource(mapping.Resource).Namespace(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", label, labelValue)})
	} else {
		list, err = k.dynClient.Resource(mapping.Resource).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", label, labelValue)})
	}

	if err != nil {
		log.Println("err", err)
		return nil
	}

	if len(list.Items) == 0 {
		return nil
	}

	return &list.Items[0]
}

func (k *k8sclient) GetObjectByName(namespace, kind, name string) *unstructured.Unstructured {
	mapping, err := k.restMapper.RESTMapping(schema.ParseGroupKind(kind))
	if err != nil {
		log.Printf("Error getting restmapping for %s: %s", kind, err)
		return nil
	}

	obj, err := k.dynClient.Resource(mapping.Resource).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})

	if err != nil {
		log.Println("err", err)
		return nil
	}

	return obj
}

func (k *k8sclient) GetObjectStatus(obj *unstructured.Unstructured) (bool, bool, error) { //need to wait, ready, error
	//TODO: somehow retrieve the table representation of the object to get serverside calculated state!
	if obj.GetObjectKind().GroupVersionKind().String() == "apps/v1, Kind=Deployment" {

		conditions := obj.Object["status"].(map[string]interface{})["conditions"]
		for _, c := range conditions.([]interface{}) {
			cT := c.(map[string]interface{})

			if cT["type"].(string) == "Available" {
				available, _ := strconv.ParseBool(cT["status"].(string))
				return true, available, nil
			}

		}

	}

	if obj.GetObjectKind().GroupVersionKind().String() == "sql.tanzu.vmware.com/v1, Kind=Postgres" {
		if obj.Object["status"] == nil {
			return true, false, nil
		}

		currentState := obj.Object["status"].(map[string]interface{})["currentState"]
		//TODO: add debug logging here.
		if currentState == "Running" {
			return true, true, nil
		} else {
			return true, false, nil
		}
	}

	return false, true, nil
}

func (k *k8sclient) DeleteFromYaml(ctx context.Context, yamlIn, instanceID string, force bool) error {
	obj, err := k.GetObject(ctx, instanceID, yamlIn)
	if err != nil {
		return err
	}

	if !k.IsNsSet(obj) {
		return nil
	}

	mapping, err := k.restMapper.RESTMapping(obj.GetObjectKind().GroupVersionKind().GroupKind(), obj.GetObjectKind().GroupVersionKind().Version)
	if err != nil {
		return err
	}

	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	deleteThisObject := true
	if !force {
		if del, ok := labels["deleteOnUnprovision"]; ok {
			delBool, err := strconv.ParseBool(del)
			if err == nil && !delBool {
				deleteThisObject = false
			}
		}
	}

	//For objects that are not being deleted during unprovision, set the "deletedDate" label. This makes cleanup process easier
	if !deleteThisObject {
		log.Printf("Skipping object deletion for object: %s because label \"deleteOnUnprovision\" is \"false\"", obj.GetName())

		labels["deletedDate"] = fmt.Sprintf("%v", time.Now().Unix())
		obj.SetLabels(labels)

		_, err := k.dynClient.Resource(mapping.Resource).Namespace(obj.GetNamespace()).Update(ctx, obj, metav1.UpdateOptions{})
		return err
	}
	err = k.dynClient.Resource(mapping.Resource).Namespace(obj.GetNamespace()).Delete(ctx, obj.GetName(), metav1.DeleteOptions{})

	return err
}
