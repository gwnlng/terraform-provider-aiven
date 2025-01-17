package schemautil

import (
	"context"
	"fmt"
	"strings"

	"github.com/aiven/aiven-go-client"
	"github.com/docker/go-units"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"golang.org/x/exp/slices"
)

func ServiceIntegrationShouldNotBeEmpty(_ context.Context, _, new, _ interface{}) bool {
	return len(new.([]interface{})) != 0
}

func DiskSpaceShouldNotBeEmpty(_ context.Context, _, new, _ interface{}) bool {
	return new.(string) != ""
}

func TagsShouldNotBeEmpty(_ context.Context, _, new, _ interface{}) bool {
	return len(new.(*schema.Set).List()) != 0
}

func CustomizeDiffServiceIntegrationAfterCreation(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	if len(d.Id()) > 0 && d.HasChange("service_integrations") && len(d.Get("service_integrations").([]interface{})) != 0 {
		return fmt.Errorf("service_integrations field can only be set during creation of a service")
	}
	return nil
}

func CustomizeDiffCheckUniqueTag(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	t := make(map[string]bool)
	for _, tag := range d.Get("tag").(*schema.Set).List() {
		tagVal := tag.(map[string]interface{})
		k := tagVal["key"].(string)
		if t[k] {
			return fmt.Errorf("tag keys should be unique, duplicate with the key: %s", k)
		}
		t[k] = true
	}

	return nil
}

func CustomizeDiffCheckDiskSpace(ctx context.Context, d *schema.ResourceDiff, m interface{}) error {
	client := m.(*aiven.Client)

	if d.Get("service_type").(string) == "" {
		return fmt.Errorf("cannot check dynamic disk space because service_type is empty")
	}

	servicePlanParams, err := GetServicePlanParametersFromSchema(ctx, client, d)
	if err != nil {
		if aiven.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("unable to get service plan parameters: %w", err)
	}

	var requestedDiskSpaceMB int

	if ds, ok := d.GetOk("disk_space"); ok {
		requestedDiskSpaceMB = ConvertToDiskSpaceMB(ds.(string))
	} else {
		if ads, ok := d.GetOk("additional_disk_space"); ok {
			requestedDiskSpaceMB = servicePlanParams.DiskSizeMBDefault + ConvertToDiskSpaceMB(ads.(string))
		}
	}

	if requestedDiskSpaceMB == 0 {
		return nil
	}

	if servicePlanParams.DiskSizeMBDefault != requestedDiskSpaceMB {
		// first check if the plan allows dynamic disk sizing
		if servicePlanParams.DiskSizeMBMax == 0 || servicePlanParams.DiskSizeMBStep == 0 {
			return fmt.Errorf("dynamic disk space is not configurable for this service")
		}

		// next check if the cloud allows it by checking the pricing per gb
		if ok, err := dynamicDiskSpaceIsAllowedByPricing(ctx, client, d); err != nil {
			return fmt.Errorf("unable to check if dynamic disk space is allowed for this service: %w", err)
		} else if !ok {
			return fmt.Errorf("dynamic disk space is not configurable for this service")
		}
	}

	humanReadableDiskSpaceDefault := HumanReadableByteSize(servicePlanParams.DiskSizeMBDefault * units.MiB)
	humanReadableDiskSpaceMax := HumanReadableByteSize(servicePlanParams.DiskSizeMBMax * units.MiB)
	humanReadableDiskSpaceStep := HumanReadableByteSize(servicePlanParams.DiskSizeMBStep * units.MiB)
	humanReadableRequestedDiskSpace := HumanReadableByteSize(requestedDiskSpaceMB * units.MiB)

	if requestedDiskSpaceMB < servicePlanParams.DiskSizeMBDefault {
		return fmt.Errorf("requested disk size is too small: '%s' < '%s'", humanReadableRequestedDiskSpace, humanReadableDiskSpaceDefault)
	}
	if servicePlanParams.DiskSizeMBMax != 0 {
		if requestedDiskSpaceMB > servicePlanParams.DiskSizeMBMax {
			return fmt.Errorf("requested disk size is too large: '%s' > '%s'", humanReadableRequestedDiskSpace, humanReadableDiskSpaceMax)
		}
	}
	if servicePlanParams.DiskSizeMBStep != 0 {
		if (requestedDiskSpaceMB-servicePlanParams.DiskSizeMBDefault)%servicePlanParams.DiskSizeMBStep != 0 {
			return fmt.Errorf("requested disk size has to increase from: '%s' in increments of '%s'", humanReadableDiskSpaceDefault, humanReadableDiskSpaceStep)
		}
	}
	return nil
}

func SetServiceTypeIfEmpty(t string) schema.CustomizeDiffFunc {
	return func(_ context.Context, diff *schema.ResourceDiff, _ interface{}) error {
		return diff.SetNew("service_type", t)
	}
}

// CustomizeDiffCheckPlanAndStaticIpsCannotBeModifiedTogether checks that 'plan' and 'static_ips'
// are not changed in the same plan, since that leads to undefined behaviour
func CustomizeDiffCheckPlanAndStaticIpsCannotBeModifiedTogether(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	if d.Id() != "" && d.HasChange("plan") && d.HasChange("static_ips") {
		return fmt.Errorf("unable to change 'plan' and 'static_ips' in the same diff, please use multiple steps")
	}
	return nil
}

// CustomizeDiffCheckStaticIPDisassociation checks that we dont disassociate ips we should not
// and are not assigning ips that are not 'created'
func CustomizeDiffCheckStaticIPDisassociation(_ context.Context, d *schema.ResourceDiff, m interface{}) error {
	contains := func(l []string, e string) bool {
		for i := range l {
			if l[i] == e {
				return true
			}
		}
		return false
	}

	if d.Id() == "" {
		return nil
	}

	client := m.(*aiven.Client)

	projectName, serviceName := d.Get("project").(string), d.Get("service_name").(string)
	var plannedStaticIps []string
	if staticIps, ok := d.GetOk("static_ips"); ok {
		plannedStaticIps = FlattenToString(staticIps.(*schema.Set).List())
	}

	resp, err := client.StaticIPs.List(projectName)
	if err != nil {
		return fmt.Errorf("unable to get static ips for project '%s': %w", projectName, err)
	}

	// Check that we block deletions that will fail because the ip belongs to the service
	for _, sip := range resp.StaticIPs {
		associatedWithDifferentService := sip.ServiceName != "" && sip.ServiceName != serviceName
		if associatedWithDifferentService && contains(plannedStaticIps, sip.StaticIPAddressID) {
			return fmt.Errorf("the static ip '%s' is currently associated with service '%s'", sip.StaticIPAddressID, sip.ServiceName)
		}

	}
	// TODO: Check that we block deletions that will result in too few static ips for the plan
	return nil
}

// CustomizeDiffDisallowMultipleManyToOneKeys checks that we don't have multiple keys that are going to be converted to
// a single key in the API request, e.g. 'ip_filter' and 'ip_filter_object' in the same diff.
func CustomizeDiffDisallowMultipleManyToOneKeys(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	for k, v := range d.GetRawConfig().AsValueMap() {
		if strings.Contains(k, "_user_config") { // we only care about *_user_config
			if err := checkForMultipleValues(v); err != nil {
				return err
			}
		}

	}

	return nil
}

// checkForMultipleValues checks for multiple values in a cty.Value
// It returns an error if multiple values are set for 'ip_filter' or 'namespaces'
func checkForMultipleValues(v cty.Value) error {
	// If v is null or empty, do not continue
	if v.IsNull() || len(v.AsValueSlice()) == 0 {
		return nil
	}

	val := v.AsValueSlice()
	// If the first element is not iterable, do not continue
	if !val[0].CanIterateElements() {
		return nil
	}

	ipFilterSetBy, namespacesSetBy := "", ""
	for k, v := range val[0].AsValueMap() {
		if v.IsNull() {
			continue
		}

		// If v is iterable and empty, skip to next iteration
		if v.CanIterateElements() && len(v.AsValueSlice()) == 0 {
			continue
		}

		// Checking for IP filters duplicates
		if slices.Contains([]string{"ip_filter", "ip_filter_string", "ip_filter_object"}, k) {
			if ipFilterSetBy != "" {
				return fmt.Errorf("cannot set '%s' and '%s'", k, ipFilterSetBy)
			}
			ipFilterSetBy = k
		}

		// Checking for namespaces duplicates
		if slices.Contains([]string{"namespaces", "namespaces_string", "namespaces_object"}, k) {
			if namespacesSetBy != "" {
				return fmt.Errorf("cannot set '%s' and '%s'", k, namespacesSetBy)
			}
			namespacesSetBy = k
		}

		// If the data structure allows going deeper recursively that do so
		if v.Type().IsListType() || v.Type().IsSetType() {
			if err := checkForMultipleValues(v); err != nil {
				return err
			}
		}
	}

	return nil
}
