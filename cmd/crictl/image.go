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

	"github.com/urfave/cli"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

var imageCommand = cli.Command{
	Name:  "image",
	Usage: "Manage image",
	Subcommands: []cli.Command{
		pullImageCommand,
		listImageCommand,
		imageStatusCommand,
		removeImageCommand,
	},
	Before: getImageClient,
	After:  closeConnection,
}

var pullImageCommand = cli.Command{
	Name:  "pull",
	Usage: "pull an image",
	Action: func(context *cli.Context) error {
		r, err := PullImage(imageClient, context.Args().Get(0))
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
			Name:  "quiet",
			Usage: "list only image IDs",
		},
	},
	Action: func(context *cli.Context) error {
		r, err := ListImages(imageClient, context.Args().Get(0))
		if err != nil {
			return fmt.Errorf("listing images failed: %v", err)
		}
		quiet := context.Bool("quiet")
		for _, image := range r.Images {
			if quiet {
				fmt.Printf("%s\n", image.Id)
				continue
			}
			fmt.Printf("ID: %s\n", image.Id)
			for _, tag := range image.RepoTags {
				fmt.Printf("Tag: %s\n", tag)
			}
			for _, digest := range image.RepoDigests {
				fmt.Printf("Digest: %s\n", digest)
			}
			if image.Size_ != 0 {
				fmt.Printf("Size: %d\n", image.Size_)
			}
			fmt.Printf("Uid: %v\n", image.Uid)
			fmt.Printf("Username: %v\n\n", image.Username)
		}
		return nil
	},
}

var imageStatusCommand = cli.Command{
	Name:  "status",
	Usage: "return the status of an image",
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		r, err := ImageStatus(imageClient, id)
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
	Name:  "rm",
	Usage: "remove an image",
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		_, err := RemoveImage(imageClient, id)
		if err != nil {
			return fmt.Errorf("removing the image failed: %v", err)
		}
		return nil
	},
}

// PullImage sends a PullImageRequest to the server, and parses
// the returned PullImageResponse.
func PullImage(client pb.ImageServiceClient, image string) (*pb.PullImageResponse, error) {
	return client.PullImage(context.Background(), &pb.PullImageRequest{Image: &pb.ImageSpec{Image: image}})
}

// ListImages sends a ListImagesRequest to the server, and parses
// the returned ListImagesResponse.
func ListImages(client pb.ImageServiceClient, image string) (*pb.ListImagesResponse, error) {
	return client.ListImages(context.Background(), &pb.ListImagesRequest{Filter: &pb.ImageFilter{Image: &pb.ImageSpec{Image: image}}})
}

// ImageStatus sends an ImageStatusRequest to the server, and parses
// the returned ImageStatusResponse.
func ImageStatus(client pb.ImageServiceClient, image string) (*pb.ImageStatusResponse, error) {
	return client.ImageStatus(context.Background(), &pb.ImageStatusRequest{Image: &pb.ImageSpec{Image: image}})
}

// RemoveImage sends a RemoveImageRequest to the server, and parses
// the returned RemoveImageResponse.
func RemoveImage(client pb.ImageServiceClient, image string) (*pb.RemoveImageResponse, error) {
	if image == "" {
		return nil, fmt.Errorf("ID cannot be empty")
	}
	return client.RemoveImage(context.Background(), &pb.RemoveImageRequest{Image: &pb.ImageSpec{Image: image}})
}
