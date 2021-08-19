package aiven

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"testing"

	"github.com/aiven/aiven-go-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccAivenKafkaACL_basic(t *testing.T) {
	resourceName := "aiven_kafka_acl.foo"
	rName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckAivenKafkaACLResourceDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testAccKafkaACLWrongProjectResource(rName),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile("invalid value for project"),
			},
			{
				Config:      testAccKafkaACLWrongServiceNameResource(rName),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile("invalid value for service_name"),
			},
			{
				Config:      testAccKafkaACLWrongPermisionResource(rName),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile("expected permission to be one of"),
			},
			{
				Config:      testAccKafkaACLWrongUsernameResource(rName),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile("invalid value for username"),
			},
			{
				Config:      testAccKafkaACLInvalidCharsResource(rName),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile("invalid value for username"),
			},
			{
				Config:             testAccKafkaACLPrefixWildcardResource(rName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config:             testAccKafkaACLWildcardResource(rName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccKafkaACLResource(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAivenKafkaACLAttributes("data.aiven_kafka_acl.acl"),
					resource.TestCheckResourceAttr(resourceName, "project", os.Getenv("AIVEN_PROJECT_NAME")),
					resource.TestCheckResourceAttr(resourceName, "service_name", fmt.Sprintf("test-acc-sr-%s", rName)),
					resource.TestCheckResourceAttr(resourceName, "topic", fmt.Sprintf("test-acc-topic-%s", rName)),
					resource.TestCheckResourceAttr(resourceName, "username", fmt.Sprintf("user-%s", rName)),
					resource.TestCheckResourceAttr(resourceName, "permission", "admin"),
				),
			},
		},
	})
}

func testAccCheckAivenKafkaACLAttributes(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		r := s.RootModule().Resources[n]
		a := r.Primary.Attributes

		log.Printf("[DEBUG] kafka acl attributes %v", a)

		if a["project"] == "" {
			return fmt.Errorf("expected to get a project name from Aiven")
		}

		if a["service_name"] == "" {
			return fmt.Errorf("expected to get a service name from Aiven")
		}

		if a["topic"] == "" {
			return fmt.Errorf("expected to get a topic from Aiven")
		}

		if a["username"] == "" {
			return fmt.Errorf("expected to get a username from Aiven")
		}

		if a["permission"] == "" {
			return fmt.Errorf("expected to get a permission from Aiven")
		}

		return nil
	}
}

func testAccKafkaACLWrongProjectResource(_ string) string {
	return `
		resource "aiven_kafka_acl" "foo" {
			project = "!./,£$^&*()_"
			service_name = "test-acc-sr-1"
			topic = "test-acc-topic-1"
			username = "user-1"
			permission = "admin"
		}
		`
}

func testAccKafkaACLWrongServiceNameResource(_ string) string {
	return `
		resource "aiven_kafka_acl" "foo" {
			project = "test-acc-pr-1"
			service_name = "!./,£$^&*()_"
			topic = "test-acc-topic-1"
			username = "user-1"
			permission = "admin"
		}
		`
}

func testAccKafkaACLWrongPermisionResource(_ string) string {
	return `
		resource "aiven_kafka_acl" "foo" {
			project = "test-acc-pr-1"
			service_name = "test-acc-sr-1"
			topic = "test-acc-topic-1"
			username = "user-1"
			permission = "wrong-permission"
		}
		`
}

func testAccKafkaACLWildcardResource(_ string) string {
	return `
		resource "aiven_kafka_acl" "foo" {
			project = "test-acc-pr-1"
			service_name = "test-acc-sr-1"
			topic = "test-acc-topic-1"
			username = "*"
			permission = "admin"
		}
		`
}

func testAccKafkaACLPrefixWildcardResource(_ string) string {
	return `
		resource "aiven_kafka_acl" "foo" {
			project = "test-acc-pr-1"
			service_name = "test-acc-sr-1"
			topic = "test-acc-topic-1"
			username = "group-user-*"
			permission = "admin"
		}
		`
}

func testAccKafkaACLWrongUsernameResource(_ string) string {
	return `
		resource "aiven_kafka_acl" "foo" {
			project = "test-acc-pr-1"
			service_name = "test-acc-sr-1"
			topic = "test-acc-topic-1"
			username = "*-user"
			permission = "admin"
		}
		`
}

func testAccKafkaACLInvalidCharsResource(_ string) string {
	return `
		resource "aiven_kafka_acl" "foo" {
			project = "test-acc-pr-1"
			service_name = "test-acc-sr-1"
			topic = "test-acc-topic-1"
			username = "!./,£$^&*()_"
			permission = "admin"
		}
		`
}

func testAccKafkaACLResource(name string) string {
	return fmt.Sprintf(`
		data "aiven_project" "foo" {
			project = "%s"
		}

		resource "aiven_service" "bar" {
			project = data.aiven_project.foo.project
			cloud_name = "google-europe-west1"
			plan = "business-4"
			service_name = "test-acc-sr-%s"
			service_type = "kafka"
			maintenance_window_dow = "monday"
			maintenance_window_time = "10:00:00"
			
			kafka_user_config {
				kafka {
				  group_max_session_timeout_ms = 70000
				  log_retention_bytes = 1000000000
				}
			}
		}
		
		resource "aiven_kafka_topic" "foo" {
			project = data.aiven_project.foo.project
			service_name = aiven_service.bar.service_name
			topic_name = "test-acc-topic-%s"
			partitions = 3
			replication = 2
		}

		resource "aiven_kafka_acl" "foo" {
			project = data.aiven_project.foo.project
			service_name = aiven_service.bar.service_name
			topic = aiven_kafka_topic.foo.topic_name
			username = "user-%s"
			permission = "admin"
		}
		
		data "aiven_kafka_acl" "acl" {
			project = aiven_kafka_acl.foo.project
			service_name = aiven_kafka_acl.foo.service_name
			topic = aiven_kafka_acl.foo.topic
			username = aiven_kafka_acl.foo.username
			permission = aiven_kafka_acl.foo.permission

			depends_on = [aiven_kafka_acl.foo]
		}
		`, os.Getenv("AIVEN_PROJECT_NAME"), name, name, name)
}

func testAccCheckAivenKafkaACLResourceDestroy(s *terraform.State) error {
	c := testAccProvider.Meta().(*aiven.Client)

	// loop through the resources in state, verifying each kafka ACL is destroyed
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aiven_kafka_acl" {
			continue
		}

		project, serviceName, aclID := splitResourceID3(rs.Primary.ID)
		p, err := c.KafkaACLs.Get(project, serviceName, aclID)
		if err != nil {
			if err.(aiven.Error).Status != 404 {
				return err
			}
		}

		if p != nil {
			return fmt.Errorf("kafka ACL (%s) still exists", rs.Primary.ID)
		}
	}

	return nil
}
