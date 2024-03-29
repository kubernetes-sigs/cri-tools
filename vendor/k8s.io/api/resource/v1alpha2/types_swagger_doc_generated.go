/*
Copyright The Kubernetes Authors.

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

package v1alpha2

// This file contains a collection of methods that can be used from go-restful to
// generate Swagger API documentation for its models. Please read this PR for more
// information on the implementation: https://github.com/emicklei/go-restful/pull/215
//
// TODOs are ignored from the parser (e.g. TODO(andronat):... || TODO:...) if and only if
// they are on one line! For multiple line or blocks that you want to ignore use ---.
// Any context after a --- is ignored.
//
// Those methods can be generated by using hack/update-codegen.sh

// AUTO-GENERATED FUNCTIONS START HERE. DO NOT EDIT.
var map_AllocationResult = map[string]string{
	"":                 "AllocationResult contains attributes of an allocated resource.",
	"resourceHandles":  "ResourceHandles contain the state associated with an allocation that should be maintained throughout the lifetime of a claim. Each ResourceHandle contains data that should be passed to a specific kubelet plugin once it lands on a node. This data is returned by the driver after a successful allocation and is opaque to Kubernetes. Driver documentation may explain to users how to interpret this data if needed.\n\nSetting this field is optional. It has a maximum size of 32 entries. If null (or empty), it is assumed this allocation will be processed by a single kubelet plugin with no ResourceHandle data attached. The name of the kubelet plugin invoked will match the DriverName set in the ResourceClaimStatus this AllocationResult is embedded in.",
	"availableOnNodes": "This field will get set by the resource driver after it has allocated the resource to inform the scheduler where it can schedule Pods using the ResourceClaim.\n\nSetting this field is optional. If null, the resource is available everywhere.",
	"shareable":        "Shareable determines whether the resource supports more than one consumer at a time.",
}

func (AllocationResult) SwaggerDoc() map[string]string {
	return map_AllocationResult
}

var map_AllocationResultModel = map[string]string{
	"":               "AllocationResultModel must have one and only one field set.",
	"namedResources": "NamedResources describes the allocation result when using the named resources model.",
}

func (AllocationResultModel) SwaggerDoc() map[string]string {
	return map_AllocationResultModel
}

var map_DriverAllocationResult = map[string]string{
	"":                        "DriverAllocationResult contains vendor parameters and the allocation result for one request.",
	"vendorRequestParameters": "VendorRequestParameters are the per-request configuration parameters from the time that the claim was allocated.",
}

func (DriverAllocationResult) SwaggerDoc() map[string]string {
	return map_DriverAllocationResult
}

var map_DriverRequests = map[string]string{
	"":                 "DriverRequests describes all resources that are needed from one particular driver.",
	"driverName":       "DriverName is the name used by the DRA driver kubelet plugin.",
	"vendorParameters": "VendorParameters are arbitrary setup parameters for all requests of the claim. They are ignored while allocating the claim.",
	"requests":         "Requests describes all resources that are needed from the driver.",
}

func (DriverRequests) SwaggerDoc() map[string]string {
	return map_DriverRequests
}

var map_NodeResourceModel = map[string]string{
	"":               "NodeResourceModel must have one and only one field set.",
	"namedResources": "NamedResources describes available resources using the named resources model.",
}

func (NodeResourceModel) SwaggerDoc() map[string]string {
	return map_NodeResourceModel
}

var map_PodSchedulingContext = map[string]string{
	"":         "PodSchedulingContext objects hold information that is needed to schedule a Pod with ResourceClaims that use \"WaitForFirstConsumer\" allocation mode.\n\nThis is an alpha type and requires enabling the DynamicResourceAllocation feature gate.",
	"metadata": "Standard object metadata",
	"spec":     "Spec describes where resources for the Pod are needed.",
	"status":   "Status describes where resources for the Pod can be allocated.",
}

func (PodSchedulingContext) SwaggerDoc() map[string]string {
	return map_PodSchedulingContext
}

var map_PodSchedulingContextList = map[string]string{
	"":         "PodSchedulingContextList is a collection of Pod scheduling objects.",
	"metadata": "Standard list metadata",
	"items":    "Items is the list of PodSchedulingContext objects.",
}

func (PodSchedulingContextList) SwaggerDoc() map[string]string {
	return map_PodSchedulingContextList
}

var map_PodSchedulingContextSpec = map[string]string{
	"":               "PodSchedulingContextSpec describes where resources for the Pod are needed.",
	"selectedNode":   "SelectedNode is the node for which allocation of ResourceClaims that are referenced by the Pod and that use \"WaitForFirstConsumer\" allocation is to be attempted.",
	"potentialNodes": "PotentialNodes lists nodes where the Pod might be able to run.\n\nThe size of this field is limited to 128. This is large enough for many clusters. Larger clusters may need more attempts to find a node that suits all pending resources. This may get increased in the future, but not reduced.",
}

func (PodSchedulingContextSpec) SwaggerDoc() map[string]string {
	return map_PodSchedulingContextSpec
}

var map_PodSchedulingContextStatus = map[string]string{
	"":               "PodSchedulingContextStatus describes where resources for the Pod can be allocated.",
	"resourceClaims": "ResourceClaims describes resource availability for each pod.spec.resourceClaim entry where the corresponding ResourceClaim uses \"WaitForFirstConsumer\" allocation mode.",
}

func (PodSchedulingContextStatus) SwaggerDoc() map[string]string {
	return map_PodSchedulingContextStatus
}

var map_ResourceClaim = map[string]string{
	"":         "ResourceClaim describes which resources are needed by a resource consumer. Its status tracks whether the resource has been allocated and what the resulting attributes are.\n\nThis is an alpha type and requires enabling the DynamicResourceAllocation feature gate.",
	"metadata": "Standard object metadata",
	"spec":     "Spec describes the desired attributes of a resource that then needs to be allocated. It can only be set once when creating the ResourceClaim.",
	"status":   "Status describes whether the resource is available and with which attributes.",
}

func (ResourceClaim) SwaggerDoc() map[string]string {
	return map_ResourceClaim
}

var map_ResourceClaimConsumerReference = map[string]string{
	"":         "ResourceClaimConsumerReference contains enough information to let you locate the consumer of a ResourceClaim. The user must be a resource in the same namespace as the ResourceClaim.",
	"apiGroup": "APIGroup is the group for the resource being referenced. It is empty for the core API. This matches the group in the APIVersion that is used when creating the resources.",
	"resource": "Resource is the type of resource being referenced, for example \"pods\".",
	"name":     "Name is the name of resource being referenced.",
	"uid":      "UID identifies exactly one incarnation of the resource.",
}

func (ResourceClaimConsumerReference) SwaggerDoc() map[string]string {
	return map_ResourceClaimConsumerReference
}

var map_ResourceClaimList = map[string]string{
	"":         "ResourceClaimList is a collection of claims.",
	"metadata": "Standard list metadata",
	"items":    "Items is the list of resource claims.",
}

func (ResourceClaimList) SwaggerDoc() map[string]string {
	return map_ResourceClaimList
}

var map_ResourceClaimParameters = map[string]string{
	"":               "ResourceClaimParameters defines resource requests for a ResourceClaim in an in-tree format understood by Kubernetes.",
	"metadata":       "Standard object metadata",
	"generatedFrom":  "If this object was created from some other resource, then this links back to that resource. This field is used to find the in-tree representation of the claim parameters when the parameter reference of the claim refers to some unknown type.",
	"shareable":      "Shareable indicates whether the allocated claim is meant to be shareable by multiple consumers at the same time.",
	"driverRequests": "DriverRequests describes all resources that are needed for the allocated claim. A single claim may use resources coming from different drivers. For each driver, this array has at most one entry which then may have one or more per-driver requests.\n\nMay be empty, in which case the claim can always be allocated.",
}

func (ResourceClaimParameters) SwaggerDoc() map[string]string {
	return map_ResourceClaimParameters
}

var map_ResourceClaimParametersList = map[string]string{
	"":         "ResourceClaimParametersList is a collection of ResourceClaimParameters.",
	"metadata": "Standard list metadata",
	"items":    "Items is the list of node resource capacity objects.",
}

func (ResourceClaimParametersList) SwaggerDoc() map[string]string {
	return map_ResourceClaimParametersList
}

var map_ResourceClaimParametersReference = map[string]string{
	"":         "ResourceClaimParametersReference contains enough information to let you locate the parameters for a ResourceClaim. The object must be in the same namespace as the ResourceClaim.",
	"apiGroup": "APIGroup is the group for the resource being referenced. It is empty for the core API. This matches the group in the APIVersion that is used when creating the resources.",
	"kind":     "Kind is the type of resource being referenced. This is the same value as in the parameter object's metadata, for example \"ConfigMap\".",
	"name":     "Name is the name of resource being referenced.",
}

func (ResourceClaimParametersReference) SwaggerDoc() map[string]string {
	return map_ResourceClaimParametersReference
}

var map_ResourceClaimSchedulingStatus = map[string]string{
	"":                "ResourceClaimSchedulingStatus contains information about one particular ResourceClaim with \"WaitForFirstConsumer\" allocation mode.",
	"name":            "Name matches the pod.spec.resourceClaims[*].Name field.",
	"unsuitableNodes": "UnsuitableNodes lists nodes that the ResourceClaim cannot be allocated for.\n\nThe size of this field is limited to 128, the same as for PodSchedulingSpec.PotentialNodes. This may get increased in the future, but not reduced.",
}

func (ResourceClaimSchedulingStatus) SwaggerDoc() map[string]string {
	return map_ResourceClaimSchedulingStatus
}

var map_ResourceClaimSpec = map[string]string{
	"":                  "ResourceClaimSpec defines how a resource is to be allocated.",
	"resourceClassName": "ResourceClassName references the driver and additional parameters via the name of a ResourceClass that was created as part of the driver deployment.",
	"parametersRef":     "ParametersRef references a separate object with arbitrary parameters that will be used by the driver when allocating a resource for the claim.\n\nThe object must be in the same namespace as the ResourceClaim.",
	"allocationMode":    "Allocation can start immediately or when a Pod wants to use the resource. \"WaitForFirstConsumer\" is the default.",
}

func (ResourceClaimSpec) SwaggerDoc() map[string]string {
	return map_ResourceClaimSpec
}

var map_ResourceClaimStatus = map[string]string{
	"":                      "ResourceClaimStatus tracks whether the resource has been allocated and what the resulting attributes are.",
	"driverName":            "DriverName is a copy of the driver name from the ResourceClass at the time when allocation started.",
	"allocation":            "Allocation is set by the resource driver once a resource or set of resources has been allocated successfully. If this is not specified, the resources have not been allocated yet.",
	"reservedFor":           "ReservedFor indicates which entities are currently allowed to use the claim. A Pod which references a ResourceClaim which is not reserved for that Pod will not be started.\n\nThere can be at most 32 such reservations. This may get increased in the future, but not reduced.",
	"deallocationRequested": "DeallocationRequested indicates that a ResourceClaim is to be deallocated.\n\nThe driver then must deallocate this claim and reset the field together with clearing the Allocation field.\n\nWhile DeallocationRequested is set, no new consumers may be added to ReservedFor.",
}

func (ResourceClaimStatus) SwaggerDoc() map[string]string {
	return map_ResourceClaimStatus
}

var map_ResourceClaimTemplate = map[string]string{
	"":         "ResourceClaimTemplate is used to produce ResourceClaim objects.",
	"metadata": "Standard object metadata",
	"spec":     "Describes the ResourceClaim that is to be generated.\n\nThis field is immutable. A ResourceClaim will get created by the control plane for a Pod when needed and then not get updated anymore.",
}

func (ResourceClaimTemplate) SwaggerDoc() map[string]string {
	return map_ResourceClaimTemplate
}

var map_ResourceClaimTemplateList = map[string]string{
	"":         "ResourceClaimTemplateList is a collection of claim templates.",
	"metadata": "Standard list metadata",
	"items":    "Items is the list of resource claim templates.",
}

func (ResourceClaimTemplateList) SwaggerDoc() map[string]string {
	return map_ResourceClaimTemplateList
}

var map_ResourceClaimTemplateSpec = map[string]string{
	"":         "ResourceClaimTemplateSpec contains the metadata and fields for a ResourceClaim.",
	"metadata": "ObjectMeta may contain labels and annotations that will be copied into the PVC when creating it. No other fields are allowed and will be rejected during validation.",
	"spec":     "Spec for the ResourceClaim. The entire content is copied unchanged into the ResourceClaim that gets created from this template. The same fields as in a ResourceClaim are also valid here.",
}

func (ResourceClaimTemplateSpec) SwaggerDoc() map[string]string {
	return map_ResourceClaimTemplateSpec
}

var map_ResourceClass = map[string]string{
	"":                     "ResourceClass is used by administrators to influence how resources are allocated.\n\nThis is an alpha type and requires enabling the DynamicResourceAllocation feature gate.",
	"metadata":             "Standard object metadata",
	"driverName":           "DriverName defines the name of the dynamic resource driver that is used for allocation of a ResourceClaim that uses this class.\n\nResource drivers have a unique name in forward domain order (acme.example.com).",
	"parametersRef":        "ParametersRef references an arbitrary separate object that may hold parameters that will be used by the driver when allocating a resource that uses this class. A dynamic resource driver can distinguish between parameters stored here and and those stored in ResourceClaimSpec.",
	"suitableNodes":        "Only nodes matching the selector will be considered by the scheduler when trying to find a Node that fits a Pod when that Pod uses a ResourceClaim that has not been allocated yet.\n\nSetting this field is optional. If null, all nodes are candidates.",
	"structuredParameters": "If and only if allocation of claims using this class is handled via structured parameters, then StructuredParameters must be set to true.",
}

func (ResourceClass) SwaggerDoc() map[string]string {
	return map_ResourceClass
}

var map_ResourceClassList = map[string]string{
	"":         "ResourceClassList is a collection of classes.",
	"metadata": "Standard list metadata",
	"items":    "Items is the list of resource classes.",
}

func (ResourceClassList) SwaggerDoc() map[string]string {
	return map_ResourceClassList
}

var map_ResourceClassParameters = map[string]string{
	"":                 "ResourceClassParameters defines resource requests for a ResourceClass in an in-tree format understood by Kubernetes.",
	"metadata":         "Standard object metadata",
	"generatedFrom":    "If this object was created from some other resource, then this links back to that resource. This field is used to find the in-tree representation of the class parameters when the parameter reference of the class refers to some unknown type.",
	"vendorParameters": "VendorParameters are arbitrary setup parameters for all claims using this class. They are ignored while allocating the claim. There must not be more than one entry per driver.",
	"filters":          "Filters describes additional contraints that must be met when using the class.",
}

func (ResourceClassParameters) SwaggerDoc() map[string]string {
	return map_ResourceClassParameters
}

var map_ResourceClassParametersList = map[string]string{
	"":         "ResourceClassParametersList is a collection of ResourceClassParameters.",
	"metadata": "Standard list metadata",
	"items":    "Items is the list of node resource capacity objects.",
}

func (ResourceClassParametersList) SwaggerDoc() map[string]string {
	return map_ResourceClassParametersList
}

var map_ResourceClassParametersReference = map[string]string{
	"":          "ResourceClassParametersReference contains enough information to let you locate the parameters for a ResourceClass.",
	"apiGroup":  "APIGroup is the group for the resource being referenced. It is empty for the core API. This matches the group in the APIVersion that is used when creating the resources.",
	"kind":      "Kind is the type of resource being referenced. This is the same value as in the parameter object's metadata.",
	"name":      "Name is the name of resource being referenced.",
	"namespace": "Namespace that contains the referenced resource. Must be empty for cluster-scoped resources and non-empty for namespaced resources.",
}

func (ResourceClassParametersReference) SwaggerDoc() map[string]string {
	return map_ResourceClassParametersReference
}

var map_ResourceFilter = map[string]string{
	"":           "ResourceFilter is a filter for resources from one particular driver.",
	"driverName": "DriverName is the name used by the DRA driver kubelet plugin.",
}

func (ResourceFilter) SwaggerDoc() map[string]string {
	return map_ResourceFilter
}

var map_ResourceFilterModel = map[string]string{
	"":               "ResourceFilterModel must have one and only one field set.",
	"namedResources": "NamedResources describes a resource filter using the named resources model.",
}

func (ResourceFilterModel) SwaggerDoc() map[string]string {
	return map_ResourceFilterModel
}

var map_ResourceHandle = map[string]string{
	"":               "ResourceHandle holds opaque resource data for processing by a specific kubelet plugin.",
	"driverName":     "DriverName specifies the name of the resource driver whose kubelet plugin should be invoked to process this ResourceHandle's data once it lands on a node. This may differ from the DriverName set in ResourceClaimStatus this ResourceHandle is embedded in.",
	"data":           "Data contains the opaque data associated with this ResourceHandle. It is set by the controller component of the resource driver whose name matches the DriverName set in the ResourceClaimStatus this ResourceHandle is embedded in. It is set at allocation time and is intended for processing by the kubelet plugin whose name matches the DriverName set in this ResourceHandle.\n\nThe maximum size of this field is 16KiB. This may get increased in the future, but not reduced.",
	"structuredData": "If StructuredData is set, then it needs to be used instead of Data.",
}

func (ResourceHandle) SwaggerDoc() map[string]string {
	return map_ResourceHandle
}

var map_ResourceRequest = map[string]string{
	"":                 "ResourceRequest is a request for resources from one particular driver.",
	"vendorParameters": "VendorParameters are arbitrary setup parameters for the requested resource. They are ignored while allocating a claim.",
}

func (ResourceRequest) SwaggerDoc() map[string]string {
	return map_ResourceRequest
}

var map_ResourceRequestModel = map[string]string{
	"":               "ResourceRequestModel must have one and only one field set.",
	"namedResources": "NamedResources describes a request for resources with the named resources model.",
}

func (ResourceRequestModel) SwaggerDoc() map[string]string {
	return map_ResourceRequestModel
}

var map_ResourceSlice = map[string]string{
	"":           "ResourceSlice provides information about available resources on individual nodes.",
	"metadata":   "Standard object metadata",
	"nodeName":   "NodeName identifies the node which provides the resources if they are local to a node.\n\nA field selector can be used to list only ResourceSlice objects with a certain node name.",
	"driverName": "DriverName identifies the DRA driver providing the capacity information. A field selector can be used to list only ResourceSlice objects with a certain driver name.",
}

func (ResourceSlice) SwaggerDoc() map[string]string {
	return map_ResourceSlice
}

var map_ResourceSliceList = map[string]string{
	"":         "ResourceSliceList is a collection of ResourceSlices.",
	"metadata": "Standard list metadata",
	"items":    "Items is the list of node resource capacity objects.",
}

func (ResourceSliceList) SwaggerDoc() map[string]string {
	return map_ResourceSliceList
}

var map_StructuredResourceHandle = map[string]string{
	"":                      "StructuredResourceHandle is the in-tree representation of the allocation result.",
	"vendorClassParameters": "VendorClassParameters are the per-claim configuration parameters from the resource class at the time that the claim was allocated.",
	"vendorClaimParameters": "VendorClaimParameters are the per-claim configuration parameters from the resource claim parameters at the time that the claim was allocated.",
	"nodeName":              "NodeName is the name of the node providing the necessary resources if the resources are local to a node.",
	"results":               "Results lists all allocated driver resources.",
}

func (StructuredResourceHandle) SwaggerDoc() map[string]string {
	return map_StructuredResourceHandle
}

var map_VendorParameters = map[string]string{
	"":           "VendorParameters are opaque parameters for one particular driver.",
	"driverName": "DriverName is the name used by the DRA driver kubelet plugin.",
	"parameters": "Parameters can be arbitrary setup parameters. They are ignored while allocating a claim.",
}

func (VendorParameters) SwaggerDoc() map[string]string {
	return map_VendorParameters
}

// AUTO-GENERATED FUNCTIONS END HERE
