package backend_mongodb

import "github.com/songzhibin97/gkit/options"

type config struct {
	// DatabaseName db名称
	databaseName string
	// TableTaskName 任务表名称
	tableTaskName string
	// TableGroupName 组表名称
	tableGroupName string
}

func SetDatabaseName(databaseName string) options.Option {
	return func(c interface{}) {
		c.(*config).databaseName = databaseName
	}
}

func SetTableTaskName(tableTaskName string) options.Option {
	return func(c interface{}) {
		c.(*config).tableTaskName = tableTaskName
	}
}

func SetTableGroupName(tableGroupName string) options.Option {
	return func(c interface{}) {
		c.(*config).tableGroupName = tableGroupName
	}
}
