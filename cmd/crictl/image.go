/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1"
)

var imageCommand = cli.Command{
	Name:  "image",
	Usage: "manage images",
	Subcommands: []cli.Command{
		pullImageCommand,
		listImageCommand,
		imageStatusCommand,
		removeImageCommand,
	},
	After: closeConnection,
}

var pullImageCommand = cli.Command{
	Name:      "pull",
	Usage:     "pull an image",
	ArgsUsage: "NAME[:TAG|@DIGEST]",
	Action: func(context *cli.Context) error {
		imageName := context.Args().First()
		if imageName == "" {
			return cli.ShowSubcommandHelp(context)
		}

		if err := getImageClient(context); err != nil {
			return err
		}

		r, err := PullImage(imageClient, imageName)
		logrus.Debugf("PullImageResponse: %v", r)
		if err != nil {
			return fmt.Errorf("pulling image failed: %v", err)
		}
		fmt.Println(r.ImageRef)
		return nil
	},
}

var listImageCommand = cli.Command{
	Name:  "ls",
	Usage: "list images",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "show verbose info for images",
		},
	},
	Action: func(context *cli.Context) error {
		if err := getImageClient(context); err != nil {
			return err
		}

		r, err := ListImages(imageClient, context.Args().First())
		logrus.Debugf("ListImagesResponse: %v", r)
		if err != nil {
			return fmt.Errorf("listing images failed: %v", err)
		}
		verbose := context.Bool("verbose")
		printHeader := true
		for _, image := range r.Images {
			if !verbose {
				if printHeader {
					printHeader = false
					fmt.Println("IMAGE\tIMAGE ID\tSIZE")
				}
				fmt.Printf("%s\t%s\t%d\n", image.RepoTags[0], image.Id, image.GetSize_())
				continue
			}
			fmt.Printf("ID: %s\n", image.Id)
			for _, tag := range image.RepoTags {
				fmt.Printf("RepoTags: %s\n", tag)
			}
			for _, digest := range image.RepoDigests {
				fmt.Printf("RepoDigests: %s\n", digest)
			}
			if image.Size_ != 0 {
				fmt.Printf("Size: %d\n", image.Size_)
			}
			if image.Uid != nil {
				fmt.Printf("Uid: %v\n", image.Uid)
			}
			if image.Username != "" {
				fmt.Printf("Username: %v\n\n", image.Username)
			}
		}
		return nil
	},
}

var imageStatusCommand = cli.Command{
	Name:      "status",
	Usage:     "return the status of an image",
	ArgsUsage: "IMAGEID",
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		if id == "" {
			return cli.ShowSubcommandHelp(context)
		}

		if err := getImageClient(context); err != nil {
			return err
		}

		r, err := ImageStatus(imageClient, id)
		logrus.Debugf("ImageStatus: %v", r)
		if err != nil {
			return fmt.Errorf("image status request failed: %v", err)
		}
		image := r.Image
		if image == nil {
			return fmt.Errorf("no such image present")
		}
		fmt.Printf("ID: %s\n", image.Id)
		for _, tag := range image.RepoTags {
			fmt.Printf("Tag: %s\n", tag)
		}
		for _, digest := range image.RepoDigests {
			fmt.Printf("Digest: %s\n", digest)
		}
		fmt.Printf("Size: %d\n", image.Size_)
		return nil
	},
}
var removeImageCommand = cli.Command{
	Name:      "rm",
	Usage:     "remove an image",
	ArgsUsage: "IMAGEID",
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		if id == "" {
			return cli.ShowSubcommandHelp(context)
		}

		if err := getImageClient(context); err != nil {
			return err
		}

		r, err := RemoveImage(imageClient, id)
		logrus.Debugf("RemoveImageResponse: %v", r)
		if err != nil {
			return fmt.Errorf("removing the image %q failed: %v", id, err)
		}
		return nil
	},
}

// PullImage sends a PullImageRequest to the server, and parses
// the returned PullImageResponse.
func PullImage(client pb.ImageServiceClient, image string) (*pb.PullImageResponse, error) {
	request := &pb.PullImageRequest{Image: &pb.ImageSpec{Image: image}}
	logrus.Debugf("PullImageRequest: %v", request)
	return client.PullImage(context.Background(), request)
}

// ListImages sends a ListImagesRequest to the server, and parses
// the returned ListImagesResponse.
func ListImages(client pb.ImageServiceClient, image string) (*pb.ListImagesResponse, error) {
	request := &pb.ListImagesRequest{Filter: &pb.ImageFilter{Image: &pb.ImageSpec{Image: image}}}
	logrus.Debugf("ListImagesRequest: %v", request)
	return client.ListImages(context.Background(), &pb.ListImagesRequest{Filter: &pb.ImageFilter{Image: &pb.ImageSpec{Image: image}}})
}

// ImageStatus sends an ImageStatusRequest to the server, and parses
// the returned ImageStatusResponse.
func ImageStatus(client pb.ImageServiceClient, image string) (*pb.ImageStatusResponse, error) {
	request := &pb.ImageStatusRequest{Image: &pb.ImageSpec{Image: image}}
	logrus.Debugf("ImageStatusRequest: %v", request)
	return client.ImageStatus(context.Background(), &pb.ImageStatusRequest{Image: &pb.ImageSpec{Image: image}})
}

// RemoveImage sends a RemoveImageRequest to the server, and parses
// the returned RemoveImageResponse.
func RemoveImage(client pb.ImageServiceClient, image string) (*pb.RemoveImageResponse, error) {
	if image == "" {
		return nil, fmt.Errorf("ImageID cannot be empty")
	}
	request := &pb.RemoveImageRequest{Image: &pb.ImageSpec{Image: image}}
	logrus.Debugf("RemoveImageRequest: %v", request)
	return client.RemoveImage(context.Background(), &pb.RemoveImageRequest{Image: &pb.ImageSpec{Image: image}})
}
