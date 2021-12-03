package sql

import (
	"github.com/neo1908/semaphore/db"
	"github.com/masterminds/squirrel"
)

func (d *SqlDb) GetRepository(projectID int, repositoryID int) (db.Repository, error) {
	var repository db.Repository
	err := d.getObject(projectID, db.RepositoryProps, repositoryID, &repository)

	if err != nil {
		return repository, err
	}

	repository.SSHKey, err = d.GetAccessKey(projectID, repository.SSHKeyID)

	return repository, err
}

func (d *SqlDb) GetRepositories(projectID int, params db.RetrieveQueryParams) (repositories []db.Repository, err error) {
	q := squirrel.Select("*").
		From("project__repository pr")

	order := "ASC"
	if params.SortInverted {
		order = "DESC"
	}

	switch params.SortBy {
	case "name", "git_url":
		q = q.Where("pr.project_id=?", projectID).
			OrderBy("pr." + params.SortBy + " " + order)
	case "ssh_key":
		q = q.LeftJoin("access_key ak ON (pr.ssh_key_id = ak.id)").
			Where("pr.project_id=?", projectID).
			OrderBy("ak.name " + order)
	default:
		q = q.Where("pr.project_id=?", projectID).
			OrderBy("pr.name " + order)
	}

	query, args, err := q.ToSql()

	if err != nil {
		return
	}

	_, err = d.selectAll(&repositories, query, args...)

	return
}

func (d *SqlDb) UpdateRepository(repository db.Repository) error {
	_, err := d.exec(
		"update project__repository set name=?, git_url=?, ssh_key_id=? where id=?",
		repository.Name,
		repository.GitURL,
		repository.SSHKeyID,
		repository.ID)

	return err
}

func (d *SqlDb) CreateRepository(repository db.Repository) (newRepo db.Repository, err error) {
	insertID, err := d.insert(
		"id",
		"insert into project__repository(project_id, git_url, ssh_key_id, name) values (?, ?, ?, ?)",
		repository.ProjectID,
		repository.GitURL,
		repository.SSHKeyID,
		repository.Name)

	if err != nil {
		return
	}

	newRepo = repository
	newRepo.ID = insertID
	return
}

func (d *SqlDb) DeleteRepository(projectID int, repositoryId int) error {
	return d.deleteObject(projectID, db.RepositoryProps, repositoryId)
}

func (d *SqlDb) DeleteRepositorySoft(projectID int, repositoryId int) error {
	return d.deleteObjectSoft(projectID, db.RepositoryProps, repositoryId)
}
