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
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
	errorUtils "k8s.io/apimachinery/pkg/util/errors"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type imageByRef []*pb.Image

func (a imageByRef) Len() int      { return len(a) }
func (a imageByRef) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a imageByRef) Less(i, j int) bool {
	if len(a[i].GetRepoTags()) > 0 && len(a[j].GetRepoTags()) > 0 {
		return a[i].GetRepoTags()[0] < a[j].GetRepoTags()[0]
	}

	if len(a[i].GetRepoDigests()) > 0 && len(a[j].GetRepoDigests()) > 0 {
		return a[i].GetRepoDigests()[0] < a[j].GetRepoDigests()[0]
	}

	return a[i].GetId() < a[j].GetId()
}

var pullImageCommand = &cli.Command{
	Name:                   "pull",
	Usage:                  "Pull an image from a registry",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "creds",
			Usage:   "Use `USERNAME[:PASSWORD]` for accessing the registry",
			EnvVars: []string{"CRICTL_CREDS"},
		},
		&cli.StringFlag{
			Name:    "auth",
			Usage:   "Use `AUTH_STRING` for accessing the registry. AUTH_STRING is a base64 encoded 'USERNAME[:PASSWORD]'",
			EnvVars: []string{"CRICTL_AUTH"},
		},
		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Usage:   "Use `USERNAME` for accessing the registry. The password will be requested on the command line",
		},
		&cli.StringFlag{
			Name:      "pod-config",
			Usage:     "Use `pod-config.[json|yaml]` to override the pull c",
			TakesFile: true,
		},
		&cli.StringSliceFlag{
			Name:    "annotation",
			Aliases: []string{"a"},
			Usage:   "Annotation to be set on the pulled image",
		},
		&cli.DurationFlag{
			Name:    "pull-timeout",
			Aliases: []string{"pt"},
			Usage:   "Maximum time to be used for pulling the image, disabled if set to 0s",
			EnvVars: []string{"CRICTL_PULL_TIMEOUT"},
		},
	},
	Subcommands: []*cli.Command{{
		Name:      "jsonschema",
		Aliases:   []string{"js"},
		Usage:     "Display the JSON schema for the pod-config.json, ",
		UsageText: "The schema will be generated from the PodSandboxConfig of the CRI API compiled with this version of crictl",
		Action: func(*cli.Context) error {
			return printJSONSchema(&pb.PodSandboxConfig{})
		},
	}},
	ArgsUsage: "NAME[:TAG|@DIGEST]",
	Action: func(c *cli.Context) error {
		imageName := c.Args().First()
		if imageName == "" {
			return errors.New("image name cannot be empty")
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
		timeout := c.Duration("pull-timeout")
		r, err := PullImageWithSandbox(c.Context, imageClient, imageName, auth, sandbox, ann, timeout)
		if err != nil {
			return fmt.Errorf("pulling image: %w", err)
		}
		fmt.Printf("Image is up to date for %s\n", r.GetImageRef())

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
		&cli.StringSliceFlag{
			Name:    "filter",
			Aliases: []string{"f"},
			Usage:   "Filter output based on provided conditions.\nAvailable filters: \n* dangling=(boolean - true or false)\n* reference=/regular expression/\n* before=<image-name>[:<tag>]|<image id>|<image@digest>\n* since=<image-name>[:<tag>]|<image id>|<image@digest>\nMultiple filters can be combined together.",
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

		r, err := ListImages(c.Context, imageClient, c.Args().First(), c.StringSlice("filter"))
		if err != nil {
			return fmt.Errorf("listing images: %w", err)
		}

		switch c.String("output") {
		case outputTypeJSON:
			return outputProtobufObjAsJSON(r)
		case outputTypeYAML:
			return outputProtobufObjAsYAML(r)
		}

		// output in table format by default.
		display := newDefaultTableDisplay()
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
		for _, image := range r.GetImages() {
			if quiet {
				fmt.Printf("%s\n", image.GetId())

				continue
			}
			if !verbose {
				imageName, repoDigest := normalizeRepoDigest(image.GetRepoDigests())
				repoTagPairs := normalizeRepoTagPair(image.GetRepoTags(), imageName)
				size := units.HumanSizeWithPrecision(float64(image.GetSize()), 3)
				id := image.GetId()
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
						row = append(row, strconv.FormatBool(image.GetPinned()))
					}
					display.AddRow(row)
				}

				continue
			}
			fmt.Printf("ID: %s\n", image.GetId())
			for _, tag := range image.GetRepoTags() {
				fmt.Printf("RepoTags: %s\n", tag)
			}
			for _, digest := range image.GetRepoDigests() {
				fmt.Printf("RepoDigests: %s\n", digest)
			}
			if image.GetSize() != 0 {
				fmt.Printf("Size: %d\n", image.GetSize())
			}
			if image.GetUid() != nil {
				fmt.Printf("Uid: %v\n", image.GetUid())
			}
			if image.GetUsername() != "" {
				fmt.Printf("Username: %v\n", image.GetUsername())
			}
			if image.GetPinned() {
				fmt.Printf("Pinned: %v\n", image.GetPinned())
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
		&cli.StringFlag{
			Name:  "name",
			Usage: "Filter by image name",
		},
		&cli.StringSliceFlag{
			Name:    "filter",
			Aliases: []string{"f"},
			Usage:   "Filter output based on provided conditions.\nAvailable filters: \n* dangling=(boolean - true or false)\n* reference=/regular expression/\n* before=<image-name>[:<tag>]|<image id>|<image@digest>\n* since=<image-name>[:<tag>]|<image id>|<image@digest>\nMultiple filters can be combined together.",
		},
	},
	Action: func(c *cli.Context) error {
		imageClient, err := getImageService(c)
		if err != nil {
			return err
		}

		verbose := !(c.Bool("quiet"))
		output := c.String("output")
		if output == "" { // default to json output
			output = outputTypeJSON
		}
		tmplStr := c.String("template")

		ids := c.Args().Slice()

		if len(ids) == 0 {
			r, err := ListImages(c.Context, imageClient, c.String("name"), c.StringSlice("filter"))
			if err != nil {
				return fmt.Errorf("listing images: %w", err)
			}
			for _, img := range r.GetImages() {
				ids = append(ids, img.GetId())
			}
		}

		if len(ids) == 0 {
			logrus.Error("No IDs provided or nothing found per filter")

			return cli.ShowSubcommandHelp(c)
		}

		statuses := []statusData{}
		for _, id := range ids {
			r, err := ImageStatus(c.Context, imageClient, id, verbose)
			if err != nil {
				return fmt.Errorf("image status for %q request: %w", id, err)
			}

			if r.GetImage() == nil {
				return fmt.Errorf("no such image %q present", id)
			}

			statusJSON, err := protobufObjectToJSON(r.GetImage())
			if err != nil {
				return fmt.Errorf("marshal status to JSON for %q: %w", id, err)
			}

			if output == outputTypeTable {
				outputImageStatusTable(r, verbose)
			} else {
				statuses = append(statuses, statusData{json: statusJSON, info: r.GetInfo()})
			}
		}

		return outputStatusData(statuses, output, tmplStr)
	},
}

func outputImageStatusTable(r *pb.ImageStatusResponse, verbose bool) {
	// otherwise output in table format
	fmt.Printf("ID: %s\n", r.GetImage().GetId())

	for _, tag := range r.GetImage().GetRepoTags() {
		fmt.Printf("Tag: %s\n", tag)
	}

	for _, digest := range r.GetImage().GetRepoDigests() {
		fmt.Printf("Digest: %s\n", digest)
	}

	size := units.HumanSizeWithPrecision(float64(r.GetImage().GetSize()), 3)
	fmt.Printf("Size: %s\n", size)

	if verbose {
		fmt.Printf("Info: %v\n", r.GetInfo())
	}
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
			r, err := InterruptableRPC(cliCtx.Context, func(ctx context.Context) ([]*pb.Image, error) {
				return imageClient.ListImages(ctx, nil)
			})
			if err != nil {
				return err
			}
			for _, img := range r {
				// Pinned images should not be removed on prune.
				if prune && img.GetPinned() {
					logrus.Debugf("Excluding pinned container image: %v", img.GetId())

					continue
				}
				logrus.Debugf("Adding container image to be removed: %v", img.GetId())
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
			containers, err := InterruptableRPC(cliCtx.Context, func(ctx context.Context) ([]*pb.Container, error) {
				return runtimeClient.ListContainers(ctx, nil)
			})
			if err != nil {
				return err
			}
			for _, container := range containers {
				img := container.GetImage().GetImage()
				imageStatus, err := ImageStatus(cliCtx.Context, imageClient, img, false)
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

		funcs := []func() error{}
		for id, remove := range ids {
			if !remove {
				continue
			}
			funcs = append(funcs, func() error {
				status, err := ImageStatus(cliCtx.Context, imageClient, id, false)
				if err != nil {
					return fmt.Errorf("image status request for %q failed: %w", id, err)
				}
				if status.GetImage() == nil {
					return fmt.Errorf("no such image %s", id)
				}

				if err := RemoveImage(cliCtx.Context, imageClient, id); err != nil {
					// We ignore further errors on prune because there might be
					// races
					if !prune {
						return fmt.Errorf("error of removing image %q: %w", id, err)
					}

					return nil
				}
				if len(status.GetImage().GetRepoTags()) == 0 {
					// RepoTags is nil when pulling image by repoDigest,
					// so print deleted using that instead.
					for _, repoDigest := range status.GetImage().GetRepoDigests() {
						fmt.Printf("Deleted: %s\n", repoDigest)
					}

					return nil
				}
				for _, repoTag := range status.GetImage().GetRepoTags() {
					fmt.Printf("Deleted: %s\n", repoTag)
				}

				return nil
			})
		}

		return errorUtils.AggregateGoroutines(funcs...)
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
			output = outputTypeJSON
		}
		tmplStr := c.String("template")

		r, err := ImageFsInfo(c.Context, imageClient)
		if err != nil {
			return fmt.Errorf("image filesystem info request: %w", err)
		}
		status, err := protobufObjectToJSON(r)
		if err != nil {
			return fmt.Errorf("marshal filesystem info to json: %w", err)
		}

		if output == outputTypeTable {
			outputImageFsInfoTable(r)
		} else {
			return outputStatusData([]statusData{{json: status}}, output, tmplStr)
		}

		return nil
	},
}

func outputImageFsInfoTable(r *pb.ImageFsInfoResponse) {
	tablePrintFileSystem := func(fileLabel string, filesystem []*pb.FilesystemUsage) {
		fmt.Printf("%s Filesystem \n", fileLabel)

		for i, val := range filesystem {
			fmt.Printf("TimeStamp[%d]: %d\n", i, val.GetTimestamp())
			fmt.Printf("Disk[%d]: %s\n", i, units.HumanSize(float64(val.GetUsedBytes().GetValue())))
			fmt.Printf("Inodes[%d]: %d\n", i, val.GetInodesUsed().GetValue())
			fmt.Printf("Mountpoint[%d]: %s\n", i, val.GetFsId().GetMountpoint())
		}
	}
	// otherwise output in table format
	tablePrintFileSystem("Container", r.GetContainerFilesystems())
	tablePrintFileSystem("Image", r.GetImageFilesystems())
}

func parseCreds(creds string) (username, password string, err error) {
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

func getAuth(creds, auth, username string) (*pb.AuthConfig, error) {
	if username != "" {
		fmt.Print("Enter Password:")

		bytePassword, err := term.ReadPassword(int(syscall.Stdin)) //nolint:unconvert // required for windows

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

func normalizeRepoDigest(repoDigests []string) (repo, digest string) {
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
func PullImageWithSandbox(ctx context.Context, client internalapi.ImageManagerService, image string, auth *pb.AuthConfig, sandbox *pb.PodSandboxConfig, ann map[string]string, timeout time.Duration) (*pb.PullImageResponse, error) {
	request := &pb.PullImageRequest{
		Image: &pb.ImageSpec{
			Image:       image,
			Annotations: ann,
		},
		Auth:          auth,
		SandboxConfig: sandbox,
	}
	logrus.Debugf("PullImageRequest: %v", request)

	if timeout < 0 {
		return nil, errors.New("timeout should be bigger than 0")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if timeout > 0 {
		logrus.Debugf("Using pull context with timeout of %s", timeout)

		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	res, err := InterruptableRPC(ctx, func(ctx context.Context) (string, error) {
		return client.PullImage(ctx, request.GetImage(), request.GetAuth(), request.GetSandboxConfig())
	})
	if err != nil {
		return nil, err
	}

	resp := &pb.PullImageResponse{ImageRef: res}
	logrus.Debugf("PullImageResponse: %v", resp)

	return resp, nil
}

// ListImages sends a ListImagesRequest to the server, and parses
// the returned ListImagesResponse.
func ListImages(ctx context.Context, client internalapi.ImageManagerService, nameFilter string, conditionFilters []string) (*pb.ListImagesResponse, error) {
	request := &pb.ListImagesRequest{Filter: &pb.ImageFilter{Image: &pb.ImageSpec{Image: nameFilter}}}
	logrus.Debugf("ListImagesRequest: %v", request)

	res, err := InterruptableRPC(ctx, func(ctx context.Context) ([]*pb.Image, error) {
		return client.ListImages(ctx, request.GetFilter())
	})
	if err != nil {
		return nil, err
	}

	resp := &pb.ListImagesResponse{Images: res}
	logrus.Debugf("ListImagesResponse: %v", resp)

	sort.Sort(imageByRef(resp.GetImages()))

	if len(conditionFilters) > 0 && len(resp.GetImages()) > 0 {
		resp.Images, err = filterImagesList(resp.GetImages(), conditionFilters)
		if err != nil {
			return nil, fmt.Errorf("filter images: %w", err)
		}
	}

	return resp, nil
}

// filterImagesList filter images based on --filter flag.
func filterImagesList(imageList []*pb.Image, filters []string) ([]*pb.Image, error) {
	filtered := []*pb.Image{}
	filtered = append(filtered, imageList...)

	for _, filter := range filters {
		switch {
		case strings.HasPrefix(filter, "before="):
			reversedList := filtered
			slices.Reverse(reversedList)
			filtered = filterByBeforeSince(strings.TrimPrefix(filter, "before="), reversedList)
			slices.Reverse(filtered)
		case strings.HasPrefix(filter, "dangling="):
			filtered = filterByDangling(strings.TrimPrefix(filter, "dangling="), filtered)
		case strings.HasPrefix(filter, "reference="):
			var err error
			if filtered, err = filterByReference(strings.TrimPrefix(filter, "reference="), filtered); err != nil {
				return []*pb.Image{}, err
			}
		case strings.HasPrefix(filter, "since="):
			filtered = filterByBeforeSince(strings.TrimPrefix(filter, "since="), filtered)
		default:
			return []*pb.Image{}, fmt.Errorf("unknown filter flag: %s", filter)
		}
	}

	return filtered, nil
}

func filterByBeforeSince(filterValue string, imageList []*pb.Image) []*pb.Image {
	filtered := []*pb.Image{}

	for _, img := range imageList {
		// Filter by <image-name>[:<tag>]
		if strings.Contains(filterValue, ":") && !strings.Contains(filterValue, "@") {
			imageName, _ := normalizeRepoDigest(img.GetRepoDigests())

			repoTagPairs := normalizeRepoTagPair(img.GetRepoTags(), imageName)
			if strings.Join(repoTagPairs[0], ":") == filterValue {
				break
			}

			filtered = append(filtered, img)
		}
		// Filter by <image id>
		if !strings.Contains(filterValue, ":") && !strings.Contains(filterValue, "@") {
			if strings.HasPrefix(img.GetId(), filterValue) {
				break
			}

			filtered = append(filtered, img)
		}
		// Filter by <image@sha>
		if strings.Contains(filterValue, ":") && strings.Contains(filterValue, "@") {
			if len(img.GetRepoDigests()) > 0 {
				if strings.HasPrefix(img.GetRepoDigests()[0], filterValue) {
					break
				}

				filtered = append(filtered, img)
			}
		}
	}

	return filtered
}

func filterByReference(filterValue string, imageList []*pb.Image) ([]*pb.Image, error) {
	filtered := []*pb.Image{}

	re, err := regexp.Compile(filterValue)
	if err != nil {
		return filtered, err
	}

	for _, img := range imageList {
		imgName, _ := normalizeRepoDigest(img.GetRepoDigests())
		if re.MatchString(imgName) || imgName == filterValue {
			filtered = append(filtered, img)
		}
	}

	return filtered, nil
}

func filterByDangling(filterValue string, imageList []*pb.Image) []*pb.Image {
	filtered := []*pb.Image{}

	for _, img := range imageList {
		if filterValue == "true" && len(img.GetRepoTags()) == 0 {
			filtered = append(filtered, img)
		}

		if filterValue == "false" && len(img.GetRepoTags()) > 0 {
			filtered = append(filtered, img)
		}
	}

	return filtered
}

// ImageStatus sends an ImageStatusRequest to the server, and parses
// the returned ImageStatusResponse.
func ImageStatus(ctx context.Context, client internalapi.ImageManagerService, image string, verbose bool) (*pb.ImageStatusResponse, error) {
	request := &pb.ImageStatusRequest{
		Image:   &pb.ImageSpec{Image: image},
		Verbose: verbose,
	}
	logrus.Debugf("ImageStatusRequest: %v", request)

	res, err := InterruptableRPC(ctx, func(ctx context.Context) (*pb.ImageStatusResponse, error) {
		return client.ImageStatus(ctx, request.GetImage(), request.GetVerbose())
	})
	if err != nil {
		return nil, err
	}

	logrus.Debugf("ImageStatusResponse: %v", res)

	return res, nil
}

// RemoveImage sends a RemoveImageRequest to the server, and parses
// the returned RemoveImageResponse.
func RemoveImage(ctx context.Context, client internalapi.ImageManagerService, image string) error {
	if image == "" {
		return errors.New("ImageID cannot be empty")
	}

	request := &pb.RemoveImageRequest{Image: &pb.ImageSpec{Image: image}}
	logrus.Debugf("RemoveImageRequest: %v", request)

	_, err := InterruptableRPC(ctx, func(ctx context.Context) (*pb.RemoveImageResponse, error) {
		return nil, client.RemoveImage(ctx, request.GetImage())
	})

	return err
}

// ImageFsInfo sends an ImageStatusRequest to the server, and parses
// the returned ImageFsInfoResponse.
func ImageFsInfo(ctx context.Context, client internalapi.ImageManagerService) (*pb.ImageFsInfoResponse, error) {
	res, err := InterruptableRPC(ctx, func(ctx context.Context) (*pb.ImageFsInfoResponse, error) {
		return client.ImageFsInfo(ctx)
	})
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
