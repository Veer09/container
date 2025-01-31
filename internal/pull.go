package image

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/urfave/cli/v3"
)

const (
	imagedbPath = "../imagedb"
	layerdbPath = "../layerdb"
)

var PullCommand = &cli.Command{
	Name:  "pull",
	Usage: "Pull an image from a docker hub",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		imageName := cmd.Args().Get(0)
		imageRef, err := name.ParseReference(imageName)
		if err != nil {
			return fmt.Errorf("error parsing image reference: %w", err)
		}
		fmt.Println("Pulling image: ", imageRef.Name())
		img, err := remote.Image(imageRef, remote.WithAuthFromKeychain(authn.DefaultKeychain))
		if err != nil {
			return fmt.Errorf("error pulling image: %w", err)
		}
		manifest, err := img.Manifest()
		if err != nil {
			return fmt.Errorf("error getting manifest: %w", err)
		}
		err = storeInImagedb(img, manifest)
		if err != nil {
			return fmt.Errorf("error storing image in db: %w", err)
		}
		for _, layer := range manifest.Layers {
			err = storeInLayerdb(layer, img)
			if err != nil {
				return fmt.Errorf("error storing layer in db: %w", err)
			}
		}
		return nil
	},
}

func storeInImagedb(img v1.Image, manifest *v1.Manifest) error {
	digest, err := img.Digest()
	if err != nil {
		return fmt.Errorf("error getting digest: %w", err)
	}
	err = os.MkdirAll(imagedbPath, 0755)
	if err != nil {
		return fmt.Errorf("error creating image directory: %w", err)
	}
	err = os.MkdirAll(layerdbPath, 0755)
	if err != nil {
		return fmt.Errorf("error creating layer directory: %w", err)
	}
	if _, err := os.Stat(imagedbPath + "/" + digest.Hex); err == nil {
		fmt.Println("Image found in cache")
		os.Exit(0)
		return nil
	}
	err = os.MkdirAll(imagedbPath+"/"+digest.Hex, 0755)
	if err != nil {
		return fmt.Errorf("error creating image directory: %w", err)
	}
	configFile, err := os.Create(imagedbPath + "/" + digest.Hex + "/config.json")
	if err != nil {
		return fmt.Errorf("error creating config file: %w", err)
	}
	defer configFile.Close()
	config, err := img.ConfigFile()
	if err != nil {
		return fmt.Errorf("error getting config: %w", err)
	}
	indentConfig, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("error indenting config: %w", err)
	}
	_, err = configFile.Write(indentConfig)
	if err != nil {
		return fmt.Errorf("error writing config: %w", err)
	}
	manifestFile, err := os.Create(imagedbPath + "/" + digest.Hex + "/manifest.json")
	if err != nil {
		return fmt.Errorf("error creating manifest file: %w", err)
	}
	defer manifestFile.Close()
	indentManifest, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("error indenting manifest: %w", err)
	}
	_, err = manifestFile.Write(indentManifest)
	if err != nil {
		return fmt.Errorf("error writing manifest: %w", err)
	}
	return nil
}

func storeInLayerdb(layer v1.Descriptor, img v1.Image) error {
	fmt.Println("Pulling layer: ", layer.Digest.Hex)
	layerDigest := layer.Digest.Hex
	_, err := os.Stat(layerdbPath + "/" + layerDigest)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error checking layer cache: %w", err)
	}
	if os.IsNotExist(err) {
		err = os.MkdirAll(layerdbPath+"/"+layerDigest, 0755)
		if err != nil {
			return fmt.Errorf("error creating layer directory: %w", err)
		}
		l, err := img.LayerByDigest(layer.Digest)
		if err != nil {
			return fmt.Errorf("error getting layer reader: %w", err)
		}
		layerReader, err := l.Uncompressed()
		if err != nil {
			return fmt.Errorf("error getting layer reader: %w", err)
		}
		defer layerReader.Close()
		tarReader := tar.NewReader(layerReader)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("error reading tar header: %w", err)
			}
			outputPath := filepath.Join(layerdbPath, layerDigest, header.Name)
			if header.Typeflag == tar.TypeDir {
				err := os.MkdirAll(outputPath, 0755)
				if err != nil {
					return fmt.Errorf("error creating directory %s: %w", outputPath, err)
				}
			} else if header.Typeflag == tar.TypeReg {
				outputFile, err := os.Create(outputPath)
				if err != nil {
					return fmt.Errorf("error creating file %s: %w", outputPath, err)
				}
				defer outputFile.Close()
				_, err = io.Copy(outputFile, tarReader)
				if err != nil {
					return fmt.Errorf("error writing file %s: %w", outputPath, err)
				}
			}
		}
		fmt.Println("Layer pulled")
	} else {
		fmt.Println("Layer found in cache")
	}
	return nil
}
