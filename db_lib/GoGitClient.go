package db_lib

import (
	"errors"
	"github.com/ansible-semaphore/semaphore/lib"

	"github.com/ansible-semaphore/semaphore/db"
	"github.com/ansible-semaphore/semaphore/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"

	ssh2 "golang.org/x/crypto/ssh"
)

type GoGitClient struct{}

type ProgressWrapper struct {
	Logger lib.Logger
}

func (t ProgressWrapper) Write(p []byte) (n int, err error) {
	t.Logger.Log(string(p))
	return len(p), nil
}

func getAuthMethod(r GitRepository) (transport.AuthMethod, error) {
	if r.Repository.SSHKey.Type == db.AccessKeySSH {
		var sshKeyBuff = r.Repository.SSHKey.SshKey.PrivateKey

		publicKey, sshErr := ssh.NewPublicKeys("git", []byte(sshKeyBuff), r.Repository.SSHKey.SshKey.Passphrase)
		if sshErr != nil {
			r.Logger.Log("Unable to creating ssh auth method")
			return nil, sshErr
		}
		publicKey.HostKeyCallback = ssh2.InsecureIgnoreHostKey()

		return publicKey, sshErr
	} else if r.Repository.SSHKey.Type == db.AccessKeyLoginPassword {
		password := &http.BasicAuth{
			Username: r.Repository.SSHKey.LoginPassword.Login,
			Password: r.Repository.SSHKey.LoginPassword.Password,
		}

		return password, nil
	} else if r.Repository.SSHKey.Type == db.AccessKeyNone {
		return nil, nil
	} else {
		return nil, errors.New("unsupported auth method")
	}
}

func openRepository(r GitRepository, targetDir GitRepositoryDirType) (*git.Repository, error) {

	var dir string

	switch targetDir {
	case GitRepositoryTmpDir:
		dir = util.Config.TmpPath
	case GitRepositoryRepoDir:
		dir = r.GetFullPath()
	default:
		panic("unknown Repository directory type")
	}

	return git.PlainOpen(dir)
}

func (c GoGitClient) Clone(r GitRepository) error {
	r.Logger.Log("Cloning Repository " + r.Repository.GitURL)

	authMethod, authErr := getAuthMethod(r)

	if authErr != nil {
		return authErr
	}

	cloneOpt := &git.CloneOptions{
		URL:               r.Repository.GetGitURL(),
		Progress:          ProgressWrapper{r.Logger},
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		ReferenceName:     plumbing.NewBranchReferenceName(r.Repository.GitBranch),
		Auth:              authMethod,
	}

	_, err := git.PlainClone(r.GetFullPath(), false, cloneOpt)
	if err != nil {
		r.Logger.Log("Unable to clone repository: " + err.Error())
	}

	return err
}

func (c GoGitClient) Pull(r GitRepository) error {
	r.Logger.Log("Updating Repository " + r.Repository.GitURL)

	rep, err := openRepository(r, GitRepositoryRepoDir)
	if err != nil {
		return err
	}

	wt, err := rep.Worktree()
	if err != nil {
		return err
	}

	authMethod, authErr := getAuthMethod(r)
	if authErr != nil {
		return authErr
	}

	// Pull the latest changes from the origin remote and merge into the current branch
	err = wt.Pull(&git.PullOptions{RemoteName: "origin", 
				       Auth: authMethod, 
				       RecurseSubmodules: git.DefaultSubmoduleRecursionDepth})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		r.Logger.Log("Unable to pull latest changes")
		return err
	}

	return nil
}

func (c GoGitClient) Checkout(r GitRepository, target string) error {
	r.Logger.Log("Checkout repository to " + target)

	rep, err := openRepository(r, GitRepositoryRepoDir)
	if err != nil {
		return err
	}

	wt, err := rep.Worktree()

	if err != nil {
		return err
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(target),
	})

	return err
}

func (c GoGitClient) CanBePulled(r GitRepository) bool {

	rep, err := openRepository(r, GitRepositoryRepoDir)
	if err != nil {
		return false
	}

	authMethod, err := getAuthMethod(r)
	if err != nil {
		return false
	}

	err = rep.Fetch(&git.FetchOptions{
		Auth: authMethod,
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		return false
	}

	head, err := rep.Head()
	if err != nil {
		return false
	}

	headCommit, err := rep.CommitObject(head.Hash())
	if err != nil {
		return false
	}

	hash, err := rep.ResolveRevision(plumbing.Revision("origin/" + r.Repository.GitBranch))
	if err != nil {
		return false
	}

	lastCommit, err := rep.CommitObject(*hash)
	if err != nil {
		return false
	}

	isAncestor, err := headCommit.IsAncestor(lastCommit)
	return isAncestor && err == nil
}

func (c GoGitClient) GetLastCommitMessage(r GitRepository) (msg string, err error) {
	r.Logger.Log("Get current commit message")

	rep, err := openRepository(r, GitRepositoryRepoDir)
	if err != nil {
		return
	}

	headRef, err := rep.Head()
	if err != nil {
		return
	}
	headCommit, err := rep.CommitObject(headRef.Hash())
	if err != nil {
		return
	}

	msg = headCommit.Message
	if len(msg) > 100 {
		msg = msg[0:100]
	}

	r.Logger.Log("Message: " + msg)

	return
}

func (c GoGitClient) GetLastCommitHash(r GitRepository) (hash string, err error) {
	r.Logger.Log("Get current commit hash")

	rep, err := openRepository(r, GitRepositoryRepoDir)
	if err != nil {
		return
	}

	headRef, err := rep.Head()
	if err != nil {
		return
	}
	hash = headRef.Hash().String()
	return
}

func (c GoGitClient) GetLastRemoteCommitHash(r GitRepository) (hash string, err error) {

	rem := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{r.Repository.GitURL},
	})

	auth, err := getAuthMethod(r)
	if err != nil {
		return
	}

	refs, err := rem.List(&git.ListOptions{
		Auth: auth,
	})
	if err != nil {
		return
	}

	var lastRemoteRef *plumbing.Reference

	for _, rf := range refs {

		if rf.Name().Short() == r.Repository.GitBranch {
			lastRemoteRef = rf
		}
	}

	if lastRemoteRef != nil {
		hash = lastRemoteRef.Hash().String()
	}

	return
}
