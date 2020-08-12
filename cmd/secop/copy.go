package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/midbel/cli"
	"github.com/midbel/toml"
	"golang.org/x/sync/semaphore"
)

func runCopy(cmd *cli.Command, args []string) error {
	quiet := cmd.Flag.Bool("q", false, "quiet")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}

	if *quiet {
		log.SetOutput(ioutil.Discard)
	}

	c := struct {
		Buffer int64
		Jobs   int64
		Credential
		Directories []Directory `toml:"directory"`
	}{}
	if err := toml.DecodeFile(cmd.Flag.Arg(0), &c); err != nil {
		return err
	}

	var err error
	if c.Jobs <= 1 {
		err = singleJob(c.Credential, c.Directories, c.Buffer)
	} else {
		err = multiJobs(c.Credential, c.Directories, c.Jobs, c.Buffer)
	}
	return err
}

func multiJobs(c Credential, dirs []Directory, jobs, buffer int64) error {
	var (
		ctx  = context.TODO()
		sema = semaphore.NewWeighted(jobs)
	)
	for i := range dirs {
		if err := sema.Acquire(ctx, 1); err != nil {
			return err
		}
		go func(d Directory) {
			defer sema.Release(1)
			if err := singleJob(c, []Directory{d}, buffer); err != nil {
				log.Println(err)
			}
		}(dirs[i])
	}

	return sema.Acquire(ctx, jobs)
}

func singleJob(c Credential, dirs []Directory, buffer int64) error {
	client, err := c.Connect()
	if err != nil {
		return err
	}
	defer func() {
		client.Close()
	}()

	for _, d := range dirs {
		if err := client.Copy(d, buffer); err != nil {
			return fmt.Errorf("fail to copy from %s to %s: %v", d.Local, d.Remote, err)
		}
	}
	return nil
}
