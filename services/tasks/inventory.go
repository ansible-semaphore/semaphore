package tasks

import (
	"io/ioutil"
	"os"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/ansible-semaphore/semaphore/db"

	"github.com/ansible-semaphore/semaphore/util"
)

func (t *TaskRunner) installInventory() (err error) {
	if t.inventory.SSHKeyID != nil {
		err = t.inventory.SSHKey.Install(db.AccessKeyRoleAnsibleUser)
		if err != nil {
			return
		}
	}

	if t.inventory.BecomeKeyID != nil {
		err = t.inventory.BecomeKey.Install(db.AccessKeyRoleAnsibleBecomeUser)
		if err != nil {
			return
		}
	}

	if t.inventory.Type == db.InventoryStatic || t.inventory.Type == db.InventoryStaticYaml {
		err = t.installStaticInventory()
	}

	return
}

func (t *TaskRunner) installStaticInventory() error {
	t.Log("installing static inventory")
	// create inventory file
	return ioutil.WriteFile(t.tmpInventoryFilename(), []byte(t.inventory.Inventory), 0664)
}

func (t *TaskRunner) tmpInventoryFilename() string {
	path := util.Config.TmpPath + "/inventory_" + strconv.Itoa(t.task.ID)
	if t.inventory.Type == db.InventoryStaticYaml {
		path += ".yml"
	}
	return path
}

func (t *TaskRunner) destroyInventoryFile() {
	tmpFileName := t.tmpInventoryFilename()
	if err := os.Remove(tmpFileName); err != nil {
		log.Error(err)
	}
}
