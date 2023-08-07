package tasks

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/ansible-semaphore/semaphore/lib"

	log "github.com/Sirupsen/logrus"
	"github.com/ansible-semaphore/semaphore/api/sockets"
	"github.com/ansible-semaphore/semaphore/db"
	"github.com/ansible-semaphore/semaphore/util"
)

type TaskRunner struct {
	task        db.Task
	template    db.Template
	inventory   db.Inventory
	repository  db.Repository
	environment db.Environment

	users     []int
	alert     bool
	alertChat *string
	prepared  bool
	process   *os.Process
	pool      *TaskPool
}

func getMD5Hash(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func (t *TaskRunner) getPlaybookDir() string {
	playbookPath := path.Join(t.getRepoPath(), t.template.Playbook)

	return path.Dir(playbookPath)
}

func (t *TaskRunner) getRepoPath() string {
	repo := lib.GitRepository{
		Logger:     t,
		TemplateID: t.template.ID,
		Repository: t.repository,
		Client:     lib.CreateDefaultGitClient(),
	}

	return repo.GetFullPath()
}

func (t *TaskRunner) setStatus(status db.TaskStatus) {
	if t.task.Status == db.TaskStoppingStatus {
		switch status {
		case db.TaskFailStatus:
			status = db.TaskStoppedStatus
		case db.TaskStoppedStatus:
		default:
			panic("stopping TaskRunner cannot be " + status)
		}
	}

	t.task.Status = status

	t.updateStatus()

	if status == db.TaskFailStatus {
		t.sendMailAlert()
	}

	if status == db.TaskSuccessStatus || status == db.TaskFailStatus {
		t.sendTelegramAlert()
		t.sendSlackAlert()
	}
}

func (t *TaskRunner) updateStatus() {
	for _, user := range t.users {
		b, err := json.Marshal(&map[string]interface{}{
			"type":        "update",
			"start":       t.task.Start,
			"end":         t.task.End,
			"status":      t.task.Status,
			"task_id":     t.task.ID,
			"template_id": t.task.TemplateID,
			"project_id":  t.task.ProjectID,
			"version":     t.task.Version,
		})

		util.LogPanic(err)

		sockets.Message(user, b)
	}

	if err := t.pool.store.UpdateTask(t.task); err != nil {
		t.panicOnError(err, "Failed to update TaskRunner status")
	}
}

func (t *TaskRunner) fail() {
	t.setStatus(db.TaskFailStatus)
}

func (t *TaskRunner) destroyKeys() {
	err := t.inventory.SSHKey.Destroy()
	if err != nil {
		t.Log("Can't destroy inventory user key, error: " + err.Error())
	}

	err = t.inventory.BecomeKey.Destroy()
	if err != nil {
		t.Log("Can't destroy inventory become user key, error: " + err.Error())
	}

	err = t.template.VaultKey.Destroy()
	if err != nil {
		t.Log("Can't destroy inventory vault password file, error: " + err.Error())
	}
}

func (t *TaskRunner) createTaskEvent() {
	objType := db.EventTask
	desc := "Task ID " + strconv.Itoa(t.task.ID) + " (" + t.template.Name + ")" + " finished - " + strings.ToUpper(string(t.task.Status))

	_, err := t.pool.store.CreateEvent(db.Event{
		UserID:      t.task.UserID,
		ProjectID:   &t.task.ProjectID,
		ObjectType:  &objType,
		ObjectID:    &t.task.ID,
		Description: &desc,
	})

	if err != nil {
		t.panicOnError(err, "Fatal error inserting an event")
	}
}

func (t *TaskRunner) prepareRun() {
	t.prepared = false

	if !t.pool.store.PermanentConnection() {
		t.pool.store.Connect("prepare task " + strconv.Itoa(t.task.ID))
		defer t.pool.store.Close("prepare task " + strconv.Itoa(t.task.ID))
	}

	defer func() {
		log.Info("Stopped preparing TaskRunner " + strconv.Itoa(t.task.ID))
		log.Info("Release resource locker with TaskRunner " + strconv.Itoa(t.task.ID))
		t.pool.resourceLocker <- &resourceLock{lock: false, holder: t}

		t.createTaskEvent()

		err := t.repository.SSHKey.Destroy()
		if err != nil {
			t.Log("Can't destroy repository access key, error: " + err.Error())
		}
	}()

	t.Log("Preparing: " + strconv.Itoa(t.task.ID))

	if err := checkTmpDir(util.Config.TmpPath); err != nil {
		t.Log("Creating tmp dir failed: " + err.Error())
		t.fail()
		return
	}

	objType := db.EventTask
	desc := "Task ID " + strconv.Itoa(t.task.ID) + " (" + t.template.Name + ")" + " is preparing"
	evt := db.Event{
		UserID:      t.task.UserID,
		ProjectID:   &t.task.ProjectID,
		ObjectType:  &objType,
		ObjectID:    &t.task.ID,
		Description: &desc,
	}

	if _, err := t.pool.store.CreateEvent(evt); err != nil {
		t.Log("Fatal error inserting an event")
		panic(err)
	}

	t.Log("Prepare TaskRunner with template: " + t.template.Name + "\n")

	t.updateStatus()

	if t.repository.GetType() == db.RepositoryLocal {
		if _, err := os.Stat(t.repository.GitURL); err != nil {
			t.Log("Failed in finding static repository at " + t.repository.GitURL + ": " + err.Error())
			t.fail()
			return
		}
	} else {
		if err := t.updateRepository(); err != nil {
			t.Log("Failed updating repository: " + err.Error())
			t.fail()
			return
		}
		if err := t.checkoutRepository(); err != nil {
			t.Log("Failed to checkout repository to required commit: " + err.Error())
			t.fail()
			return
		}
	}

	if err := t.installInventory(); err != nil {
		t.Log("Failed to install inventory: " + err.Error())
		t.fail()
		return
	}

	if err := t.installRequirements(); err != nil {
		t.Log("Running galaxy failed: " + err.Error())
		t.fail()
		return
	}

	if err := t.installVaultKeyFile(); err != nil {
		t.Log("Failed to install vault password file: " + err.Error())
		t.fail()
		return
	}

	t.prepared = true
}

func (t *TaskRunner) run() {
	if !t.pool.store.PermanentConnection() {
		t.pool.store.Connect("run task " + strconv.Itoa(t.task.ID))
		defer t.pool.store.Close("run task " + strconv.Itoa(t.task.ID))
	}

	defer func() {
		log.Info("Stopped running TaskRunner " + strconv.Itoa(t.task.ID))
		log.Info("Release resource locker with TaskRunner " + strconv.Itoa(t.task.ID))
		t.pool.resourceLocker <- &resourceLock{lock: false, holder: t}

		now := time.Now()
		t.task.End = &now
		t.updateStatus()
		t.createTaskEvent()
		t.destroyKeys()
		t.destroyInventoryFile()
	}()

	// TODO: more details
	if t.task.Status == db.TaskStoppingStatus {
		t.setStatus(db.TaskStoppedStatus)
		return
	}

	now := time.Now()
	t.task.Start = &now
	t.setStatus(db.TaskRunningStatus)

	objType := db.EventTask
	desc := "Task ID " + strconv.Itoa(t.task.ID) + " (" + t.template.Name + ")" + " is running"

	_, err := t.pool.store.CreateEvent(db.Event{
		UserID:      t.task.UserID,
		ProjectID:   &t.task.ProjectID,
		ObjectType:  &objType,
		ObjectID:    &t.task.ID,
		Description: &desc,
	})

	if err != nil {
		t.Log("Fatal error inserting an event")
		panic(err)
	}

	t.Log("Started: " + strconv.Itoa(t.task.ID))
	t.Log("Run TaskRunner with template: " + t.template.Name + "\n")

	// Mark task as stopped if user stops task during preparation (before task run).
	if t.task.Status == db.TaskStoppingStatus {
		t.setStatus(db.TaskStoppedStatus)
		return
	}

	err = t.runPlaybook()
	if err != nil {
		t.Log("Running playbook failed: " + err.Error())
		t.fail()
		return
	}

	t.setStatus(db.TaskSuccessStatus)

	templates, err := t.pool.store.GetTemplates(t.task.ProjectID, db.TemplateFilter{
		BuildTemplateID: &t.task.TemplateID,
		AutorunOnly:     true,
	}, db.RetrieveQueryParams{})
	if err != nil {
		t.Log("Running playbook failed: " + err.Error())
		return
	}

	for _, tpl := range templates {
		_, err = t.pool.AddTask(db.Task{
			TemplateID:  tpl.ID,
			ProjectID:   tpl.ProjectID,
			BuildTaskID: &t.task.ID,
		}, nil, tpl.ProjectID)
		if err != nil {
			t.Log("Running playbook failed: " + err.Error())
			continue
		}
	}
}

func (t *TaskRunner) prepareError(err error, errMsg string) error {
	if err == db.ErrNotFound {
		t.Log(errMsg)
		return err
	}

	if err != nil {
		t.fail()
		panic(err)
	}

	return nil
}

// nolint: gocyclo
func (t *TaskRunner) populateDetails() error {
	// get template
	var err error

	t.template, err = t.pool.store.GetTemplate(t.task.ProjectID, t.task.TemplateID)
	if err != nil {
		return t.prepareError(err, "Template not found!")
	}

	// get project alert setting
	project, err := t.pool.store.GetProject(t.template.ProjectID)
	if err != nil {
		return t.prepareError(err, "Project not found!")
	}

	t.alert = project.Alert
	t.alertChat = project.AlertChat

	// get project users
	users, err := t.pool.store.GetProjectUsers(t.template.ProjectID, db.RetrieveQueryParams{})
	if err != nil {
		return t.prepareError(err, "Users not found!")
	}

	t.users = []int{}
	for _, user := range users {
		t.users = append(t.users, user.ID)
	}

	// get inventory
	t.inventory, err = t.pool.store.GetInventory(t.template.ProjectID, t.template.InventoryID)
	if err != nil {
		return t.prepareError(err, "Template Inventory not found!")
	}

	// get repository
	t.repository, err = t.pool.store.GetRepository(t.template.ProjectID, t.template.RepositoryID)

	if err != nil {
		return err
	}

	err = t.repository.SSHKey.DeserializeSecret()
	if err != nil {
		return err
	}

	// get environment
	if t.template.EnvironmentID != nil {
		t.environment, err = t.pool.store.GetEnvironment(t.template.ProjectID, *t.template.EnvironmentID)
		if err != nil {
			return err
		}
	}

	if t.task.Environment != "" {
		environment := make(map[string]interface{})
		if t.environment.JSON != "" {
			err = json.Unmarshal([]byte(t.task.Environment), &environment)
			if err != nil {
				return err
			}
		}

		taskEnvironment := make(map[string]interface{})
		err = json.Unmarshal([]byte(t.environment.JSON), &taskEnvironment)
		if err != nil {
			return err
		}

		for k, v := range taskEnvironment {
			environment[k] = v
		}

		var ev []byte
		ev, err = json.Marshal(environment)
		if err != nil {
			return err
		}

		t.environment.JSON = string(ev)
	}

	return nil
}

func (t *TaskRunner) installVaultKeyFile() error {
	if t.template.VaultKeyID == nil {
		return nil
	}

	return t.template.VaultKey.Install(db.AccessKeyRoleAnsiblePasswordVault)
}

func (t *TaskRunner) checkoutRepository() error {

	repo := lib.GitRepository{
		Logger:     t,
		TemplateID: t.template.ID,
		Repository: t.repository,
		Client:     lib.CreateDefaultGitClient(),
	}

	err := repo.ValidateRepo()

	if err != nil {
		return err
	}

	if t.task.CommitHash != nil {
		// checkout to commit if it is provided for TaskRunner
		return repo.Checkout(*t.task.CommitHash)
	}

	// store commit to TaskRunner table

	commitHash, err := repo.GetLastCommitHash()

	if err != nil {
		return err
	}

	commitMessage, _ := repo.GetLastCommitMessage()

	t.task.CommitHash = &commitHash
	t.task.CommitMessage = commitMessage

	return t.pool.store.UpdateTask(t.task)
}

func (t *TaskRunner) updateRepository() error {
	repo := lib.GitRepository{
		Logger:     t,
		TemplateID: t.template.ID,
		Repository: t.repository,
		Client:     lib.CreateDefaultGitClient(),
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

func (t *TaskRunner) installCollectionsRequirements() error {
	requirementsFilePath := path.Join(t.getPlaybookDir(), "collections", "requirements.yml")
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

func (t *TaskRunner) installRolesRequirements() error {
	requirementsFilePath := fmt.Sprintf("%s/roles/requirements.yml", t.getRepoPath())
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

func (t *TaskRunner) installRequirements() error {
	if err := t.installCollectionsRequirements(); err != nil {
		return err
	}
	if err := t.installRolesRequirements(); err != nil {
		return err
	}
	return nil
}

func (t *TaskRunner) runGalaxy(args []string) error {
	return lib.AnsiblePlaybook{
		Logger:     t,
		TemplateID: t.template.ID,
		Repository: t.repository,
	}.RunGalaxy(args)
}

func (t *TaskRunner) runPlaybook() (err error) {
	args, err := t.getPlaybookArgs()
	if err != nil {
		return
	}

	environmentVariables, err := t.getEnvironmentENV()
	if err != nil {
		return
	}

	return lib.AnsiblePlaybook{
		Logger:     t,
		TemplateID: t.template.ID,
		Repository: t.repository,
	}.RunPlaybook(args, &environmentVariables, func(p *os.Process) { t.process = p })
}

func (t *TaskRunner) getEnvironmentENV() (arr []string, err error) {
	environmentVars := make(map[string]string)

	if t.environment.ENV != nil {
		err = json.Unmarshal([]byte(*t.environment.ENV), &environmentVars)
		if err != nil {
			return
		}
	}

	for key, val := range environmentVars {
		arr = append(arr, fmt.Sprintf("%s=%s", key, val))
	}

	return
}

func (t *TaskRunner) getEnvironmentExtraVars() (str string, err error) {
	extraVars := make(map[string]interface{})

	if t.environment.JSON != "" {
		err = json.Unmarshal([]byte(t.environment.JSON), &extraVars)
		if err != nil {
			return
		}
	}

	taskDetails := make(map[string]interface{})
	
	taskDetails["id"] = t.task.ID

	if t.task.Message != "" {
		taskDetails["message"] = t.task.Message
	}

	if t.task.UserID != nil {
		var user db.User
		user, err = t.pool.store.GetUser(*t.task.UserID)
		if err == nil {
			taskDetails["username"] = user.Username
		}
	}

	if t.template.Type != db.TemplateTask {
		taskDetails["type"] = t.template.Type
		incomingVersion := t.task.GetIncomingVersion(t.pool.store)
		if incomingVersion != nil {
			taskDetails["incoming_version"] = incomingVersion
		}
		if t.template.Type == db.TemplateBuild {
			taskDetails["target_version"] = t.task.Version
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

// nolint: gocyclo
func (t *TaskRunner) getPlaybookArgs() (args []string, err error) {
	playbookName := t.task.Playbook
	if playbookName == "" {
		playbookName = t.template.Playbook
	}

	var inventory string
	switch t.inventory.Type {
	case db.InventoryFile:
		inventory = t.inventory.Inventory
	case db.InventoryStatic, db.InventoryStaticYaml:
		inventory = t.tmpInventoryFilename()
	default:
		err = fmt.Errorf("invalid inventory type")
		return
	}

	args = []string{
		"-i", inventory,
	}

	if t.inventory.SSHKeyID != nil {
		switch t.inventory.SSHKey.Type {
		case db.AccessKeySSH:
			args = append(args, "--private-key="+t.inventory.SSHKey.GetPath())
			//args = append(args, "--extra-vars={\"ansible_ssh_private_key_file\": \""+t.inventory.SSHKey.GetPath()+"\"}")
			if t.inventory.SSHKey.SshKey.Login != "" {
				args = append(args, "--extra-vars={\"ansible_user\": \""+t.inventory.SSHKey.SshKey.Login+"\"}")
			}
		case db.AccessKeyLoginPassword:
			args = append(args, "--extra-vars=@"+t.inventory.SSHKey.GetPath())
		case db.AccessKeyNone:
		default:
			err = fmt.Errorf("access key does not suite for inventory's user credentials")
			return
		}
	}

	if t.inventory.BecomeKeyID != nil {
		switch t.inventory.BecomeKey.Type {
		case db.AccessKeyLoginPassword:
			args = append(args, "--extra-vars=@"+t.inventory.BecomeKey.GetPath())
		case db.AccessKeyNone:
		default:
			err = fmt.Errorf("access key does not suite for inventory's sudo user credentials")
			return
		}
	}

	if t.task.Debug {
		args = append(args, "-vvvv")
	}

	if t.task.Diff {
		args = append(args, "--diff")
	}

	if t.task.DryRun {
		args = append(args, "--check")
	}

	if t.template.VaultKeyID != nil {
		args = append(args, "--vault-password-file", t.template.VaultKey.GetPath())
	}

	extraVars, err := t.getEnvironmentExtraVars()
	if err != nil {
		t.Log(err.Error())
		t.Log("Could not remove command environment, if existant it will be passed to --extra-vars. This is not fatal but be aware of side effects")
	} else if extraVars != "" {
		args = append(args, "--extra-vars", extraVars)
	}

	var templateExtraArgs []string
	if t.template.Arguments != nil {
		err = json.Unmarshal([]byte(*t.template.Arguments), &templateExtraArgs)
		if err != nil {
			t.Log("Invalid format of the template extra arguments, must be valid JSON")
			return
		}
	}

	var taskExtraArgs []string
	if t.template.AllowOverrideArgsInTask && t.task.Arguments != nil {
		err = json.Unmarshal([]byte(*t.task.Arguments), &taskExtraArgs)
		if err != nil {
			t.Log("Invalid format of the TaskRunner extra arguments, must be valid JSON")
			return
		}
	}

	if t.task.Limit != "" {
		t.Log("--limit=" + t.task.Limit)
		taskExtraArgs = append(taskExtraArgs, "--limit="+t.task.Limit)
	}

	args = append(args, templateExtraArgs...)
	args = append(args, taskExtraArgs...)
	args = append(args, playbookName)

	return
}

func hasRequirementsChanges(requirementsFilePath string, requirementsHashFilePath string) bool {
	oldFileMD5HashBytes, err := ioutil.ReadFile(requirementsHashFilePath)
	if err != nil {
		return true
	}

	newFileMD5Hash, err := getMD5Hash(requirementsFilePath)
	if err != nil {
		return true
	}

	return string(oldFileMD5HashBytes) != newFileMD5Hash
}

func writeMD5Hash(requirementsFile string, requirementsHashFile string) error {
	newFileMD5Hash, err := getMD5Hash(requirementsFile)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(requirementsHashFile, []byte(newFileMD5Hash), 0644)
}

// checkTmpDir checks to see if the temporary directory exists
// and if it does not attempts to create it
func checkTmpDir(path string) error {
	var err error
	if _, err = os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(path, 0700)
		}
	}
	return err
}
