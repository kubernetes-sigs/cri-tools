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
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"syscall"

	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type imageByRef []*pb.Image

func (a imageByRef) Len() int      { return len(a) }
func (a imageByRef) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a imageByRef) Less(i, j int) bool {
	if len(a[i].RepoTags) > 0 && len(a[j].RepoTags) > 0 {
		return a[i].RepoTags[0] < a[j].RepoTags[0]
	}
	if len(a[i].RepoDigests) > 0 && len(a[j].RepoDigests) > 0 {
		return a[i].RepoDigests[0] < a[j].RepoDigests[0]
	}
	return a[i].Id < a[j].Id
}

var pullImageCommand = &cli.Command{
	Name:                   "pull",
	Usage:                  "Pull an image from a registry",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "creds",
			Value:   "",
			Usage:   "Use `USERNAME[:PASSWORD]` for accessing the registry",
			EnvVars: []string{"CRICTL_CREDS"},
		},
		&cli.StringFlag{
			Name:    "auth",
			Value:   "",
			Usage:   "Use `AUTH_STRING` for accessing the registry. AUTH_STRING is a base64 encoded 'USERNAME[:PASSWORD]'",
			EnvVars: []string{"CRICTL_AUTH"},
		},
		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Value:   "",
			Usage:   "Use `USERNAME` for accessing the registry. The password will be requested on the command line",
		},
		&cli.StringFlag{
			Name:      "pod-config",
			Value:     "",
			Usage:     "Use `pod-config.[json|yaml]` to override the pull c",
			TakesFile: true,
		},
		&cli.StringSliceFlag{
			Name:    "annotation",
			Aliases: []string{"a"},
			Usage:   "Annotation to be set on the pulled image",
		},
	},
	ArgsUsage: "NAME[:TAG|@DIGEST]",
	Action: func(c *cli.Context) error {
		imageName := c.Args().First()
		if imageName == "" {
			return fmt.Errorf("Image name cannot be empty")
		}

		if c.NArg() > 1 {
			return cli.ShowSubcommandHelp(c)
		}

		imageClient, err := getImageService(c)
		if err != nil {
			return err
		}

		auth, err := getAuth(c.String("creds"), c.String("auth"), c.String("username"))
		if err != nil {
			return err
		}
		var sandbox *pb.PodSandboxConfig
		if c.IsSet("pod-config") {
			sandbox, err = loadPodSandboxConfig(c.String("pod-config"))
			if err != nil {
				return fmt.Errorf("load podSandboxConfig: %w", err)
			}
		}
		var ann map[string]string
		if c.IsSet("annotation") {
			annotationFlags := c.StringSlice("annotation")
			ann, err = parseLabelStringSlice(annotationFlags)
			if err != nil {
				return err
			}
		}
		r, err := PullImageWithSandbox(imageClient, imageName, auth, sandbox, ann)
		if err != nil {
			return fmt.Errorf("pulling image: %w", err)
		}
		fmt.Printf("Image is up to date for %s\n", r.ImageRef)
		return nil
	},
}

var listImageCommand = &cli.Command{
	Name:                   "images",
	Aliases:                []string{"image", "img"},
	Usage:                  "List images",
	ArgsUsage:              "[REPOSITORY[:TAG]]",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "Show verbose info for images",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Only show image IDs",
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output format, One of: json|yaml|table",
		},
		&cli.BoolFlag{
			Name:  "digests",
			Usage: "Show digests",
		},
		&cli.BoolFlag{
			Name:  "no-trunc",
			Usage: "Show output without truncating the ID",
		},
		&cli.BoolFlag{
			Name:  "pinned",
			Usage: "Show whether the image is pinned or not",
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() > 1 {
			return cli.ShowSubcommandHelp(c)
		}

		imageClient, err := getImageService(c)
		if err != nil {
			return err
		}

		r, err := ListImages(imageClient, c.Args().First())
		if err != nil {
			return fmt.Errorf("listing images: %w", err)
		}
		sort.Sort(imageByRef(r.Images))

		switch c.String("output") {
		case "json":
			return outputProtobufObjAsJSON(r)
		case "yaml":
			return outputProtobufObjAsYAML(r)
		}

		// output in table format by default.
		display := newTableDisplay(20, 1, 3, ' ', 0)
		verbose := c.Bool("verbose")
		showDigest := c.Bool("digests")
		showPinned := c.Bool("pinned")
		quiet := c.Bool("quiet")
		noTrunc := c.Bool("no-trunc")
		if !verbose && !quiet {
			row := []string{columnImage, columnTag}
			if showDigest {
				row = append(row, columnDigest)
			}
			row = append(row, columnImageID, columnSize)
			if showPinned {
				row = append(row, columnPinned)
			}
			display.AddRow(row)
		}
		for _, image := range r.Images {
			if quiet {
				fmt.Printf("%s\n", image.Id)
				continue
			}
			if !verbose {
				imageName, repoDigest := normalizeRepoDigest(image.RepoDigests)
				repoTagPairs := normalizeRepoTagPair(image.RepoTags, imageName)
				size := units.HumanSizeWithPrecision(float64(image.GetSize_()), 3)
				id := image.Id
				if !noTrunc {
					id = getTruncatedID(id, "sha256:")
					repoDigest = getTruncatedID(repoDigest, "sha256:")
				}
				for _, repoTagPair := range repoTagPairs {
					row := []string{repoTagPair[0], repoTagPair[1]}
					if showDigest {
						row = append(row, repoDigest)
					}
					row = append(row, id, size)
					if showPinned {
						row = append(row, fmt.Sprintf("%v", image.Pinned))
					}
					display.AddRow(row)
				}
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
				fmt.Printf("Username: %v\n", image.Username)
			}
			if image.Pinned {
				fmt.Printf("Pinned: %v\n", image.Pinned)
			}
			fmt.Printf("\n")
		}
		display.Flush()
		return nil
	},
}

var imageStatusCommand = &cli.Command{
	Name:                   "inspecti",
	Usage:                  "Return the status of one or more images",
	ArgsUsage:              "IMAGE-ID [IMAGE-ID...]",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output format, One of: json|yaml|go-template|table",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Do not show verbose information",
		},
		&cli.StringFlag{
			Name:  "template",
			Usage: "The template string is only used when output is go-template; The Template format is golang template",
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() == 0 {
			return cli.ShowSubcommandHelp(c)
		}
		imageClient, err := getImageService(c)
		if err != nil {
			return err
		}

		verbose := !(c.Bool("quiet"))
		output := c.String("output")
		if output == "" { // default to json output
			output = "json"
		}
		tmplStr := c.String("template")
		for i := 0; i < c.NArg(); i++ {
			id := c.Args().Get(i)

			r, err := ImageStatus(imageClient, id, verbose)
			if err != nil {
				return fmt.Errorf("image status for %q request: %w", id, err)
			}
			image := r.Image
			if image == nil {
				return fmt.Errorf("no such image %q present", id)
			}

			status, err := protobufObjectToJSON(r.Image)
			if err != nil {
				return fmt.Errorf("marshal status to json for %q: %w", id, err)
			}
			switch output {
			case "json", "yaml", "go-template":
				if err := outputStatusInfo(status, r.Info, output, tmplStr); err != nil {
					return fmt.Errorf("output status for %q: %w", id, err)
				}
				continue
			case "table": // table output is after this switch block
			default:
				return fmt.Errorf("output option cannot be %s", output)
			}

			// otherwise output in table format
			fmt.Printf("ID: %s\n", image.Id)
			for _, tag := range image.RepoTags {
				fmt.Printf("Tag: %s\n", tag)
			}
			for _, digest := range image.RepoDigests {
				fmt.Printf("Digest: %s\n", digest)
			}
			size := units.HumanSizeWithPrecision(float64(image.GetSize_()), 3)
			fmt.Printf("Size: %s\n", size)
			if verbose {
				fmt.Printf("Info: %v\n", r.GetInfo())
			}
		}

		return nil
	},
}

var removeImageCommand = &cli.Command{
	Name:                   "rmi",
	Usage:                  "Remove one or more images",
	ArgsUsage:              "IMAGE-ID [IMAGE-ID...]",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "all",
			Aliases: []string{"a"},
			Usage:   "Remove all images",
		},
		&cli.BoolFlag{
			Name:    "prune",
			Aliases: []string{"q"},
			Usage:   "Remove all unused images",
		},
	},
	Action: func(cliCtx *cli.Context) error {
		imageClient, err := getImageService(cliCtx)
		if err != nil {
			return err
		}

		ids := map[string]bool{}
		for _, id := range cliCtx.Args().Slice() {
			logrus.Debugf("User specified image to be removed: %v", id)
			ids[id] = true
		}

		all := cliCtx.Bool("all")
		prune := cliCtx.Bool("prune")

		// Add all available images to the ID selector
		if all || prune {
			r, err := imageClient.ListImages(context.TODO(), nil)
			if err != nil {
				return err
			}
			for _, img := range r {
				logrus.Debugf("Adding image to be removed: %v", img.GetId())
				ids[img.GetId()] = true
			}
		}

		// On prune, remove images which are in use from the ID selector
		if prune {
			runtimeClient, err := getRuntimeService(cliCtx, 0)
			if err != nil {
				return err
			}

			// Container images
			containers, err := runtimeClient.ListContainers(context.TODO(), nil)
			if err != nil {
				return err
			}
			for _, container := range containers {
				img := container.GetImage().Image
				imageStatus, err := ImageStatus(imageClient, img, false)
				if err != nil {
					logrus.Errorf(
						"image status request for %q failed: %v",
						img, err,
					)
					continue
				}
				id := imageStatus.GetImage().GetId()
				logrus.Debugf("Excluding in use container image: %v", id)
				ids[id] = false
			}
		}

		if len(ids) == 0 {
			if all || prune {
				logrus.Info("No images to remove")
				return nil
			}
			return cli.ShowSubcommandHelp(cliCtx)
		}

		errored := false
		for id, remove := range ids {
			if !remove {
				continue
			}
			status, err := ImageStatus(imageClient, id, false)
			if err != nil {
				logrus.Errorf("image status request for %q failed: %v", id, err)
				errored = true
				continue
			}
			if status.Image == nil {
				logrus.Errorf("no such image %s", id)
				errored = true
				continue
			}

			if err := RemoveImage(imageClient, id); err != nil {
				// We ignore further errors on prune because there might be
				// races
				if !prune {
					logrus.Errorf("error of removing image %q: %v", id, err)
					errored = true
				}
				continue
			}
			if len(status.Image.RepoTags) == 0 {
				// RepoTags is nil when pulling image by repoDigest,
				// so print deleted using that instead.
				for _, repoDigest := range status.Image.RepoDigests {
					fmt.Printf("Deleted: %s\n", repoDigest)
				}
				continue
			}
			for _, repoTag := range status.Image.RepoTags {
				fmt.Printf("Deleted: %s\n", repoTag)
			}
		}

		if errored {
			return fmt.Errorf("unable to remove the image(s)")
		}

		return nil
	},
}

var imageFsInfoCommand = &cli.Command{
	Name:                   "imagefsinfo",
	Usage:                  "Return image filesystem info",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output format, One of: json|yaml|go-template|table",
		},
		&cli.StringFlag{
			Name:  "template",
			Usage: "The template string is only used when output is go-template; The Template format is golang template",
		},
	},
	Action: func(c *cli.Context) error {
		imageClient, err := getImageService(c)
		if err != nil {
			return err
		}

		output := c.String("output")
		if output == "" { // default to json output
			output = "json"
		}
		tmplStr := c.String("template")

		r, err := ImageFsInfo(imageClient)
		if err != nil {
			return fmt.Errorf("image filesystem info request: %w", err)
		}
		fileSystemReport := func(filesystem []*pb.FilesystemUsage) error {
			for _, info := range filesystem {
				status, err := protobufObjectToJSON(info)
				if err != nil {
					return fmt.Errorf("marshal filesystem info to json: %w", err)
				}

				switch output {
				case "json", "yaml", "go-template":
					if err := outputStatusInfo(status, nil, output, tmplStr); err != nil {
						return fmt.Errorf("output filesystem info: %w", err)
					}
					continue
				case "table": // table output is after this switch block
				default:
					return fmt.Errorf("output option cannot be %s", output)
				}

				// otherwise output in table format
				fmt.Printf("TimeStamp: %d\n", info.Timestamp)
				fmt.Printf("Disk: %s\n", units.HumanSize(float64(info.UsedBytes.GetValue())))
				fmt.Printf("Inodes: %d\n", info.InodesUsed.GetValue())
				fmt.Printf("Mountpoint: %s\n", info.FsId.Mountpoint)
			}
			return nil
		}
		if err := fileSystemReport(r.ImageFilesystems); err != nil {
			return err
		}
		if err := fileSystemReport(r.ContainerFilesystems); err != nil {
			return err
		}

		return nil

	},
}

func parseCreds(creds string) (string, string, error) {
	if creds == "" {
		return "", "", errors.New("credentials can't be empty")
	}
	up := strings.SplitN(creds, ":", 2)
	if len(up) == 1 {
		return up[0], "", nil
	}
	if up[0] == "" {
		return "", "", errors.New("username can't be empty")
	}
	return up[0], up[1], nil
}

func getAuth(creds string, auth string, username string) (*pb.AuthConfig, error) {
	if username != "" {
		fmt.Print("Enter Password:")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Print("\n")
		if err != nil {
			return nil, err
		}
		password := string(bytePassword)
		return &pb.AuthConfig{
			Username: username,
			Password: password,
		}, nil
	}
	if creds != "" && auth != "" {
		return nil, errors.New("both `--creds` and `--auth` are specified")
	}
	if creds != "" {
		username, password, err := parseCreds(creds)
		if err != nil {
			return nil, err
		}
		return &pb.AuthConfig{
			Username: username,
			Password: password,
		}, nil
	}
	if auth != "" {
		return &pb.AuthConfig{
			Auth: auth,
		}, nil
	}
	return nil, nil
}

// Ideally repo tag should always be image:tag.
// The repoTags is nil when pulling image by repoDigest,Then we will show image name instead.
func normalizeRepoTagPair(repoTags []string, imageName string) (repoTagPairs [][]string) {
	const none = "<none>"
	if len(repoTags) == 0 {
		repoTagPairs = append(repoTagPairs, []string{imageName, none})
		return
	}
	for _, repoTag := range repoTags {
		idx := strings.LastIndex(repoTag, ":")
		if idx == -1 {
			repoTagPairs = append(repoTagPairs, []string{"errorRepoTag", "errorRepoTag"})
			continue
		}
		name := repoTag[:idx]
		if name == none {
			name = imageName
		}
		repoTagPairs = append(repoTagPairs, []string{name, repoTag[idx+1:]})
	}
	return
}

func normalizeRepoDigest(repoDigests []string) (string, string) {
	if len(repoDigests) == 0 {
		return "<none>", "<none>"
	}
	repoDigestPair := strings.Split(repoDigests[0], "@")
	if len(repoDigestPair) != 2 {
		return "errorName", "errorRepoDigest"
	}
	return repoDigestPair[0], repoDigestPair[1]
}

// PullImageWithSandbox sends a PullImageRequest to the server, and parses
// the returned PullImageResponse.
func PullImageWithSandbox(client internalapi.ImageManagerService, image string, auth *pb.AuthConfig, sandbox *pb.PodSandboxConfig, ann map[string]string) (*pb.PullImageResponse, error) {
	request := &pb.PullImageRequest{
		Image: &pb.ImageSpec{
			Image:       image,
			Annotations: ann,
		},
	}
	if auth != nil {
		request.Auth = auth
	}
	if sandbox != nil {
		request.SandboxConfig = sandbox
	}
	logrus.Debugf("PullImageRequest: %v", request)
	res, err := client.PullImage(context.TODO(), request.Image, request.Auth, request.SandboxConfig)
	if err != nil {
		return nil, err
	}
	resp := &pb.PullImageResponse{ImageRef: res}
	logrus.Debugf("PullImageResponse: %v", resp)
	return resp, nil
}

// ListImages sends a ListImagesRequest to the server, and parses
// the returned ListImagesResponse.
func ListImages(client internalapi.ImageManagerService, image string) (*pb.ListImagesResponse, error) {
	request := &pb.ListImagesRequest{Filter: &pb.ImageFilter{Image: &pb.ImageSpec{Image: image}}}
	logrus.Debugf("ListImagesRequest: %v", request)
	res, err := client.ListImages(context.TODO(), request.Filter)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListImagesResponse{Images: res}
	logrus.Debugf("ListImagesResponse: %v", resp)
	return resp, nil
}

// ImageStatus sends an ImageStatusRequest to the server, and parses
// the returned ImageStatusResponse.
func ImageStatus(client internalapi.ImageManagerService, image string, verbose bool) (*pb.ImageStatusResponse, error) {
	request := &pb.ImageStatusRequest{
		Image:   &pb.ImageSpec{Image: image},
		Verbose: verbose,
	}
	logrus.Debugf("ImageStatusRequest: %v", request)
	res, err := client.ImageStatus(context.TODO(), request.Image, request.Verbose)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("ImageStatusResponse: %v", res)
	return res, nil
}

// RemoveImage sends a RemoveImageRequest to the server, and parses
// the returned RemoveImageResponse.
func RemoveImage(client internalapi.ImageManagerService, image string) error {
	if image == "" {
		return fmt.Errorf("ImageID cannot be empty")
	}
	request := &pb.RemoveImageRequest{Image: &pb.ImageSpec{Image: image}}
	logrus.Debugf("RemoveImageRequest: %v", request)
	if err := client.RemoveImage(context.TODO(), request.Image); err != nil {
		return err
	}
	return nil
}

// ImageFsInfo sends an ImageStatusRequest to the server, and parses
// the returned ImageFsInfoResponse.
func ImageFsInfo(client internalapi.ImageManagerService) (*pb.ImageFsInfoResponse, error) {
	res, err := client.ImageFsInfo(context.TODO())
	if err != nil {
		return nil, err
	}
	resp := &pb.ImageFsInfoResponse{
		ImageFilesystems:     res.GetImageFilesystems(),
		ContainerFilesystems: res.GetContainerFilesystems(),
	}
	logrus.Debugf("ImageFsInfoResponse: %v", resp)
	return resp, nil
}
