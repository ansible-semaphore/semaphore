package tasks

import (
	"encoding/json"
	"fmt"
	"github.com/ansible-semaphore/semaphore/lib"
	"os"
	"path"
	"strconv"

	"github.com/ansible-semaphore/semaphore/db"
	"github.com/ansible-semaphore/semaphore/db_lib"
	"github.com/ansible-semaphore/semaphore/util"
)

type LocalJob struct {
	// Received constant fields
	Task        db.Task
	Template    db.Template
	Inventory   db.Inventory
	Repository  db.Repository
	Environment db.Environment
	Playbook    *db_lib.AnsiblePlaybook
	Logger      lib.Logger

	// Internal field
	Process *os.Process

	sshKeyInstallation    db.AccessKeyInstallation
	becomeKeyInstallation db.AccessKeyInstallation
	vaultFileInstallation db.AccessKeyInstallation
}

func (t *LocalJob) Kill() {
	if t.Process == nil {
		return
	}
	err := t.Process.Kill()
	if err != nil {
		t.Log(err.Error())
	}
}

func (t *LocalJob) Log(msg string) {
	t.Logger.Log(msg)
}

func (t *LocalJob) SetStatus(status lib.TaskStatus) {
	t.Logger.SetStatus(status)
}

func (t *LocalJob) getEnvironmentExtraVars(username string, incomingVersion *string) (str string, err error) {
	extraVars := make(map[string]interface{})

	if t.Environment.JSON != "" {
		err = json.Unmarshal([]byte(t.Environment.JSON), &extraVars)
		if err != nil {
			return
		}
	}

	taskDetails := make(map[string]interface{})

	taskDetails["id"] = t.Task.ID

	if t.Task.Message != "" {
		taskDetails["message"] = t.Task.Message
	}

	taskDetails["username"] = username

	if t.Template.Type != db.TemplateTask {
		taskDetails["type"] = t.Template.Type
		if incomingVersion != nil {
			taskDetails["incoming_version"] = incomingVersion
		}
		if t.Template.Type == db.TemplateBuild {
			taskDetails["target_version"] = t.Task.Version
		}
	}

	vars := make(map[string]interface{})
	vars["task_details"] = taskDetails
	extraVars["semaphore_vars"] = vars

	ev, err := json.Marshal(extraVars)
	if err != nil {
		return
	}

	str = string(ev)

	return
}

func (t *LocalJob) getEnvironmentENV() (arr []string, err error) {
	environmentVars := make(map[string]string)

	if t.Environment.ENV != nil {
		err = json.Unmarshal([]byte(*t.Environment.ENV), &environmentVars)
		if err != nil {
			return
		}
	}

	for key, val := range environmentVars {
		arr = append(arr, fmt.Sprintf("%s=%s", key, val))
	}

	return
}

// nolint: gocyclo
func (t *LocalJob) getPlaybookArgs(username string, incomingVersion *string) (args []string, err error) {
	playbookName := t.Task.Playbook
	if playbookName == "" {
		playbookName = t.Template.Playbook
	}

	var inventory string
	switch t.Inventory.Type {
	case db.InventoryFile:
		inventory = t.Inventory.Inventory
	case db.InventoryStatic, db.InventoryStaticYaml:
		inventory = util.Config.TmpPath + "/inventory_" + strconv.Itoa(t.Task.ID)
		if t.Inventory.Type == db.InventoryStaticYaml {
			inventory += ".yml"
		}
	default:
		err = fmt.Errorf("invalid invetory type")
		return
	}

	args = []string{
		"-i", inventory,
	}

	if t.Inventory.SSHKeyID != nil {
		switch t.Inventory.SSHKey.Type {
		case db.AccessKeySSH:
			//args = append(args, "--extra-vars={\"ansible_ssh_private_key_file\": \""+t.inventory.SSHKey.GetPath()+"\"}")
			if t.Inventory.SSHKey.SshKey.Login != "" {
				args = append(args, "--extra-vars={\"ansible_user\": \""+t.Inventory.SSHKey.SshKey.Login+"\"}")
			}
		case db.AccessKeyLoginPassword:
			args = append(args, "--extra-vars=@"+t.sshKeyInstallation.GetPath())
		case db.AccessKeyNone:
		default:
			err = fmt.Errorf("access key does not suite for inventory's user credentials")
			return
		}
	}

	if t.Inventory.BecomeKeyID != nil {
		switch t.Inventory.BecomeKey.Type {
		case db.AccessKeyLoginPassword:
			args = append(args, "--extra-vars=@"+t.becomeKeyInstallation.GetPath())
		case db.AccessKeyNone:
		default:
			err = fmt.Errorf("access key does not suite for inventory's sudo user credentials")
			return
		}
	}

	if t.Task.Debug {
		args = append(args, "-vvvv")
	}

	if t.Task.Diff {
		args = append(args, "--diff")
	}

	if t.Task.DryRun {
		args = append(args, "--check")
	}

	if t.Template.VaultKeyID != nil {
		args = append(args, "--vault-password-file", t.vaultFileInstallation.GetPath())
	}

	extraVars, err := t.getEnvironmentExtraVars(username, incomingVersion)
	if err != nil {
		t.Log(err.Error())
		t.Log("Could not remove command environment, if existant it will be passed to --extra-vars. This is not fatal but be aware of side effects")
	} else if extraVars != "" {
		args = append(args, "--extra-vars", extraVars)
	}

	var templateExtraArgs []string
	if t.Template.Arguments != nil {
		err = json.Unmarshal([]byte(*t.Template.Arguments), &templateExtraArgs)
		if err != nil {
			t.Log("Invalid format of the template extra arguments, must be valid JSON")
			return
		}
	}

	var taskExtraArgs []string
	if t.Template.AllowOverrideArgsInTask && t.Task.Arguments != nil {
		err = json.Unmarshal([]byte(*t.Task.Arguments), &taskExtraArgs)
		if err != nil {
			t.Log("Invalid format of the TaskRunner extra arguments, must be valid JSON")
			return
		}
	}

	if t.Task.Limit != "" {
		t.Log("--limit=" + t.Task.Limit)
		taskExtraArgs = append(taskExtraArgs, "--limit="+t.Task.Limit)
	}

	args = append(args, templateExtraArgs...)
	args = append(args, taskExtraArgs...)
	args = append(args, playbookName)

	return
}

func (t *LocalJob) Run(username string, incomingVersion *string) (err error) {

	t.SetStatus(lib.TaskRunningStatus)

	err = t.prepareRun()
	if err != nil {
		return err
	}

	defer func() {
		t.destroyKeys()
	}()

	args, err := t.getPlaybookArgs(username, incomingVersion)
	if err != nil {
		return
	}

	environmentVariables, err := t.getEnvironmentENV()
	if err != nil {
		return
	}

	if t.Inventory.SSHKey.Type == db.AccessKeySSH && t.Inventory.SSHKeyID != nil {

		//var sshAgent lib.SshAgent
		//sshAgent, err = t.Inventory.StartSshAgent(t.Logger)
		//
		//if err != nil {
		//	return
		//}
		//
		//defer sshAgent.Close()

		environmentVariables = append(environmentVariables, fmt.Sprintf("SSH_AUTH_SOCK=%s", t.sshKeyInstallation.SshAgent.SocketFile))
	}

	return t.Playbook.RunPlaybook(args, &environmentVariables, func(p *os.Process) {
		t.Process = p
	})

}

func (t *LocalJob) prepareRun() error {
	t.Log("Preparing: " + strconv.Itoa(t.Task.ID))

	if err := checkTmpDir(util.Config.TmpPath); err != nil {
		t.Log("Creating tmp dir failed: " + err.Error())
		return err
	}

	if t.Repository.GetType() == db.RepositoryLocal {
		if _, err := os.Stat(t.Repository.GitURL); err != nil {
			t.Log("Failed in finding static repository at " + t.Repository.GitURL + ": " + err.Error())
			return err
		}
	} else {
		if err := t.updateRepository(); err != nil {
			t.Log("Failed updating repository: " + err.Error())
			return err
		}
		if err := t.checkoutRepository(); err != nil {
			t.Log("Failed to checkout repository to required commit: " + err.Error())
			return err
		}
	}

	if err := t.installInventory(); err != nil {
		t.Log("Failed to install inventory: " + err.Error())
		return err
	}

	if err := t.installRequirements(); err != nil {
		t.Log("Running galaxy failed: " + err.Error())
		return err
	}

	if err := t.installVaultKeyFile(); err != nil {
		t.Log("Failed to install vault password file: " + err.Error())
		return err
	}

	return nil
}

func (t *LocalJob) updateRepository() error {
	repo := db_lib.GitRepository{
		Logger:     t.Logger,
		TemplateID: t.Template.ID,
		Repository: t.Repository,
		Client:     db_lib.CreateDefaultGitClient(),
	}

	err := repo.ValidateRepo()

	if err != nil {
		if !os.IsNotExist(err) {
			err = os.RemoveAll(repo.GetFullPath())
			if err != nil {
				return err
			}
		}
		return repo.Clone()
	}

	if repo.CanBePulled() {
		err = repo.Pull()
		if err == nil {
			return nil
		}
	}

	err = os.RemoveAll(repo.GetFullPath())
	if err != nil {
		return err
	}

	return repo.Clone()
}

func (t *LocalJob) checkoutRepository() error {

	repo := db_lib.GitRepository{
		Logger:     t.Logger,
		TemplateID: t.Template.ID,
		Repository: t.Repository,
		Client:     db_lib.CreateDefaultGitClient(),
	}

	err := repo.ValidateRepo()

	if err != nil {
		return err
	}

	if t.Task.CommitHash != nil {
		// checkout to commit if it is provided for TaskRunner
		return repo.Checkout(*t.Task.CommitHash)
	}

	// store commit to TaskRunner table

	//commitHash, err := repo.GetLastCommitHash()
	//
	//if err != nil {
	//	return err
	//}
	//
	//commitMessage, _ := repo.GetLastCommitMessage()
	//
	//t.task.CommitHash = &commitHash
	//t.task.CommitMessage = commitMessage
	//
	//return t.pool.store.UpdateTask(t.task)
	return nil
}

func (t *LocalJob) installRequirements() error {
	if err := t.installCollectionsRequirements(); err != nil {
		return err
	}
	if err := t.installRolesRequirements(); err != nil {
		return err
	}
	return nil
}

func (t *LocalJob) getRepoPath() string {
	repo := db_lib.GitRepository{
		Logger:     t.Logger,
		TemplateID: t.Template.ID,
		Repository: t.Repository,
		Client:     db_lib.CreateDefaultGitClient(),
	}

	return repo.GetFullPath()
}

func (t *LocalJob) installRolesRequirements() error {
	requirementsFilePath := path.Join(t.getRepoPath(), "roles", "requirements.yml")
	requirementsHashFilePath := fmt.Sprintf("%s.md5", requirementsFilePath)

	if _, err := os.Stat(requirementsFilePath); err != nil {
		t.Log("No roles/requirements.yml file found. Skip galaxy install process.\n")
		return nil
	}

	if hasRequirementsChanges(requirementsFilePath, requirementsHashFilePath) {
		if err := t.runGalaxy([]string{
			"role",
			"install",
			"-r",
			requirementsFilePath,
			"--force",
		}); err != nil {
			return err
		}
		if err := writeMD5Hash(requirementsFilePath, requirementsHashFilePath); err != nil {
			return err
		}
	} else {
		t.Log("roles/requirements.yml has no changes. Skip galaxy install process.\n")
	}

	return nil
}

func (t *LocalJob) getPlaybookDir() string {
	playbookPath := path.Join(t.getRepoPath(), t.Template.Playbook)

	return path.Dir(playbookPath)
}

func (t *LocalJob) installCollectionsRequirements() error {
	requirementsFilePath := path.Join(t.getRepoPath(), "collections", "requirements.yml")
	requirementsHashFilePath := fmt.Sprintf("%s.md5", requirementsFilePath)

	if _, err := os.Stat(requirementsFilePath); err != nil {
		t.Log("No collections/requirements.yml file found. Skip galaxy install process.\n")
		return nil
	}

	if hasRequirementsChanges(requirementsFilePath, requirementsHashFilePath) {
		if err := t.runGalaxy([]string{
			"collection",
			"install",
			"-r",
			requirementsFilePath,
			"--force",
		}); err != nil {
			return err
		}
		if err := writeMD5Hash(requirementsFilePath, requirementsHashFilePath); err != nil {
			return err
		}
	} else {
		t.Log("collections/requirements.yml has no changes. Skip galaxy install process.\n")
	}

	return nil
}

func (t *LocalJob) runGalaxy(args []string) error {
	return t.Playbook.RunGalaxy(args)
}

func (t *LocalJob) installVaultKeyFile() (err error) {
	if t.Template.VaultKeyID == nil {
		return nil
	}

	t.vaultFileInstallation, err = t.Template.VaultKey.Install(db.AccessKeyRoleAnsiblePasswordVault, t.Logger)

	return
}
