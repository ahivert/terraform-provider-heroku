package heroku

import (
	"fmt"
	"log"
	"sync"

	"github.com/cyberdelia/heroku-go/v3"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

// Global lock to prevent parallelism for heroku_addon since
// the Heroku API cannot handle a single application requesting
// multiple addons simultaneously.
var addonLock sync.Mutex

func resourceHerokuAddon() *schema.Resource {
	return &schema.Resource{
		Create: resourceHerokuAddonCreate,
		Read:   resourceHerokuAddonRead,
		Update: resourceHerokuAddonUpdate,
		Delete: resourceHerokuAddonDelete,

		Schema: map[string]*schema.Schema{
			"app": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"plan": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},

			"config": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Schema{
					Type: schema.TypeMap,
				},
			},

			"provider_id": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},

			"config_vars": &schema.Schema{
				Type:     schema.TypeList,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeMap},
			},
		},
	}
}

func resourceHerokuAddonCreate(d *schema.ResourceData, meta interface{}) error {
	addonLock.Lock()
	defer addonLock.Unlock()

	client := meta.(*heroku.Service)

	app := d.Get("app").(string)
	opts := heroku.AddonCreateOpts{Plan: d.Get("plan").(string)}

	if v := d.Get("config"); v != nil {
		config := make(map[string]string)
		for _, v := range v.([]interface{}) {
			for k, v := range v.(map[string]interface{}) {
				config[k] = v.(string)
			}
		}

		opts.Config = &config
	}

	log.Printf("[DEBUG] Addon create configuration: %#v, %#v", app, opts)
	a, err := client.AddonCreate(app, opts)
	if err != nil {
		return err
	}

	d.SetId(a.ID)
	log.Printf("[INFO] Addon ID: %s", d.Id())

	return resourceHerokuAddonRead(d, meta)
}

func resourceHerokuAddonRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*heroku.Service)

	addon, err := resource_heroku_addon_retrieve(
		d.Get("app").(string), d.Id(), client)
	if err != nil {
		return err
	}

	d.Set("name", addon.Name)
	d.Set("plan", addon.Plan.Name)
	d.Set("provider_id", addon.ProviderID)
	d.Set("config_vars", []interface{}{addon.ConfigVars})
	d.SetDependencies([]terraform.ResourceDependency{
		terraform.ResourceDependency{ID: d.Get("app").(string)},
	})

	return nil
}

func resourceHerokuAddonUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*heroku.Service)

	app := d.Get("app").(string)

	if d.HasChange("plan") {
		ad, err := client.AddonUpdate(
			app, d.Id(), heroku.AddonUpdateOpts{Plan: d.Get("plan").(string)})
		if err != nil {
			return err
		}

		// Store the new ID
		d.SetId(ad.ID)
	}

	return resourceHerokuAddonRead(d, meta)
}

func resourceHerokuAddonDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*heroku.Service)

	log.Printf("[INFO] Deleting Addon: %s", d.Id())

	// Destroy the app
	err := client.AddonDelete(d.Get("app").(string), d.Id())
	if err != nil {
		return fmt.Errorf("Error deleting addon: %s", err)
	}

	d.SetId("")
	return nil
}

func resource_heroku_addon_retrieve(app string, id string, client *heroku.Service) (*heroku.Addon, error) {
	addon, err := client.AddonInfo(app, id)

	if err != nil {
		return nil, fmt.Errorf("Error retrieving addon: %s", err)
	}

	return addon, nil
}
