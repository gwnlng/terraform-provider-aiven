---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "aiven_gcp_vpc_peering_connection Resource - terraform-provider-aiven"
subcategory: ""
description: |-
  The GCP VPC Peering Connection resource allows the creation and management of Aiven GCP VPC Peering Connections.
---

# aiven_gcp_vpc_peering_connection (Resource)

The GCP VPC Peering Connection resource allows the creation and management of Aiven GCP VPC Peering Connections.

## Example Usage

```terraform
resource "aiven_gcp_vpc_peering_connection" "foo" {
  vpc_id         = data.aiven_project_vpc.vpc.id
  gcp_project_id = "xxxx"
  peer_vpc       = "xxxx"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- **gcp_project_id** (String) GCP project ID. This property cannot be changed, doing so forces recreation of the resource.
- **peer_vpc** (String) GCP VPC network name. This property cannot be changed, doing so forces recreation of the resource.
- **vpc_id** (String) The VPC the peering connection belongs to. This property cannot be changed, doing so forces recreation of the resource.

### Optional

- **id** (String) The ID of this resource.
- **timeouts** (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))

### Read-Only

- **state** (String) State of the peering connection
- **state_info** (Map of String) State-specific help or error information

<a id="nestedblock--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- **create** (String)
- **delete** (String)

## Import

Import is supported using the following syntax:

```shell
terraform import aiven_gcp_vpc_peering_connection.foo project_name/vpc_id/gcp_project_id/peer_vpc
```