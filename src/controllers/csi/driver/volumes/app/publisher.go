/*
Copyright 2021 Dynatrace LLC.

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

package appvolumes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	dtcsi "github.com/Dynatrace/dynatrace-operator/src/controllers/csi"
	csivolumes "github.com/Dynatrace/dynatrace-operator/src/controllers/csi/driver/volumes"
	"github.com/Dynatrace/dynatrace-operator/src/controllers/csi/metadata"
	"github.com/container-storage-interface/spec/lib/go/csi"
	dto "github.com/prometheus/client_model/go"
	"github.com/spf13/afero"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/mount"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewAppVolumePublisher(client client.Client, fs afero.Afero, mounter mount.Interface, db metadata.Access, path metadata.PathResolver) csivolumes.Publisher {
	return &AppVolumePublisher{
		client:  client,
		fs:      fs,
		mounter: mounter,
		db:      db,
		path:    path,
	}
}

type AppVolumePublisher struct {
	client  client.Client
	fs      afero.Afero
	mounter mount.Interface
	db      metadata.Access
	path    metadata.PathResolver
}

func (publisher *AppVolumePublisher) PublishVolume(ctx context.Context, volumeCfg *csivolumes.VolumeConfig) (*csi.NodePublishVolumeResponse, error) {
	bindCfg, err := csivolumes.NewBindConfig(ctx, publisher.db, volumeCfg)
	if err != nil {
		return nil, err
	}

	if err := publisher.mountOneAgent(bindCfg, volumeCfg); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to mount oneagent volume: %s", err))
	}

	if err := publisher.storeVolume(bindCfg, volumeCfg); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("Failed to store volume info: %s", err))
	}
	agentsVersionsMetric.WithLabelValues(bindCfg.Version).Inc()
	return &csi.NodePublishVolumeResponse{}, nil
}

func (publisher *AppVolumePublisher) UnpublishVolume(_ context.Context, volumeInfo *csivolumes.VolumeInfo) (*csi.NodeUnpublishVolumeResponse, error) {
	volume, err := publisher.loadVolume(volumeInfo.VolumeID)
	if err != nil {
		log.Info("failed to load volume info", "error", err.Error())
	}
	if volume == nil {
		return nil, nil
	}
	log.Info("loaded volume info", "id", volume.VolumeID, "pod name", volume.PodName, "version", volume.Version, "dynakube", volume.TenantUUID)

	overlayFSPath := publisher.path.AgentRunDirForVolume(volume.TenantUUID, volumeInfo.VolumeID)

	if err = publisher.umountOneAgent(volumeInfo.TargetPath, overlayFSPath); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to unmount oneagent volume: %s", err.Error()))
	}

	if err = publisher.db.DeleteVolume(volume.VolumeID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	log.Info("deleted volume info", "ID", volume.VolumeID, "PodUID", volume.PodName, "Version", volume.Version, "TenantUUID", volume.TenantUUID)

	if err = publisher.fs.RemoveAll(volumeInfo.TargetPath); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.Info("volume has been unpublished", "targetPath", volumeInfo.TargetPath)

	publisher.fireVolumeUnpublishedMetric(*volume)

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (publisher *AppVolumePublisher) CanUnpublishVolume(volumeInfo *csivolumes.VolumeInfo) (bool, error) {
	volume, err := publisher.loadVolume(volumeInfo.VolumeID)
	if err != nil {
		return false, status.Error(codes.Internal, fmt.Sprintf("failed to get volume info from database: %s", err.Error()))
	}
	return volume != nil, nil
}

func (publisher *AppVolumePublisher) fireVolumeUnpublishedMetric(volume metadata.Volume) {
	if len(volume.Version) > 0 {
		agentsVersionsMetric.WithLabelValues(volume.Version).Dec()
		var m = &dto.Metric{}
		if err := agentsVersionsMetric.WithLabelValues(volume.Version).Write(m); err != nil {
			log.Error(err, "failed to get the value of agent version metric")
		}
		if m.Gauge.GetValue() <= float64(0) {
			agentsVersionsMetric.DeleteLabelValues(volume.Version)
		}
	}
}

func (publisher *AppVolumePublisher) mountOneAgent(bindCfg *csivolumes.BindConfig, volumeCfg *csivolumes.VolumeConfig) error {
	mappedDir := publisher.path.OverlayMappedDir(bindCfg.TenantUUID, volumeCfg.VolumeID)
	_ = publisher.fs.MkdirAll(mappedDir, os.ModePerm)

	upperDir := publisher.path.OverlayVarDir(bindCfg.TenantUUID, volumeCfg.VolumeID)
	_ = publisher.fs.MkdirAll(upperDir, os.ModePerm)

	workDir := publisher.path.OverlayWorkDir(bindCfg.TenantUUID, volumeCfg.VolumeID)
	_ = publisher.fs.MkdirAll(workDir, os.ModePerm)

	overlayOptions := []string{
		"lowerdir=" + publisher.path.AgentBinaryDirForVersion(bindCfg.TenantUUID, bindCfg.Version),
		"upperdir=" + upperDir,
		"workdir=" + workDir,
	}

	if err := publisher.fs.MkdirAll(volumeCfg.TargetPath, os.ModePerm); err != nil {
		return err
	}

	if err := publisher.mounter.Mount("overlay", mappedDir, "overlay", overlayOptions); err != nil {
		return err
	}
	if err := publisher.mounter.Mount(mappedDir, volumeCfg.TargetPath, "", []string{"bind"}); err != nil {
		_ = publisher.mounter.Unmount(mappedDir)
		return err
	}

	return nil
}

func (publisher *AppVolumePublisher) umountOneAgent(targetPath string, overlayFSPath string) error {
	if err := publisher.mounter.Unmount(targetPath); err != nil {
		log.Error(err, "Unmount failed", "path", targetPath)
	}

	if filepath.IsAbs(overlayFSPath) {
		agentDirectoryForPod := filepath.Join(overlayFSPath, dtcsi.OverlayMappedDirPath)
		if err := publisher.mounter.Unmount(agentDirectoryForPod); err != nil {
			log.Error(err, "Unmount failed", "path", agentDirectoryForPod)
		}
	}

	return nil
}

func (publisher *AppVolumePublisher) storeVolume(bindCfg *csivolumes.BindConfig, volumeCfg *csivolumes.VolumeConfig) error {
	volume := metadata.NewVolume(volumeCfg.VolumeID, volumeCfg.PodName, bindCfg.Version, bindCfg.TenantUUID)
	log.Info("inserting volume info", "ID", volume.VolumeID, "PodUID", volume.PodName, "Version", volume.Version, "TenantUUID", volume.TenantUUID)
	return publisher.db.InsertVolume(volume)
}

func (publisher *AppVolumePublisher) loadVolume(volumeID string) (*metadata.Volume, error) {
	volume, err := publisher.db.GetVolume(volumeID)
	if err != nil {
		return nil, err
	}
	return volume, nil
}
