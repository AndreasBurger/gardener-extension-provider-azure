// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"

	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

func getStorageAccountName(backupBucket *extensionsv1alpha1.BackupBucket) string {
	return fmt.Sprintf("bkp%s", utils.ComputeSHA256Hex([]byte(backupBucket.Name))[:15])
}

func ensureBackupBucket(ctx context.Context, factory azureclient.Factory, backupBucket *extensionsv1alpha1.BackupBucket) (string, string, error) {
	storageAccountName := getStorageAccountName(backupBucket)

	// Get resource group client to ensure resource group to host backup storage account exists.
	groupClient, err := factory.Group()
	if err != nil {
		return "", "", err
	}
	if _, err := groupClient.CreateOrUpdate(ctx, backupBucket.Name, armresources.ResourceGroup{
		Location: to.Ptr(backupBucket.Spec.Region),
	}); err != nil {
		return "", "", err
	}

	// Get storage account client to create the backup storage account.
	storageAccountClient, err := factory.StorageAccount()
	if err != nil {
		return "", "", err
	}
	if err := storageAccountClient.CreateStorageAccount(ctx, backupBucket.Name, storageAccountName, backupBucket.Spec.Region); err != nil {
		return "", "", err
	}

	// Get a key of the storage account. We simply use the first one for new storage accounts (later rotations may change the "current" one).
	storageAccountKey, err := storageAccountClient.ListStorageAccountKey(ctx, backupBucket.Name, storageAccountName)
	if err != nil {
		return "", "", err
	}

	return storageAccountName, storageAccountKey, nil
}

func rotateStorageAccountCredentials(
	ctx context.Context,
	factory azureclient.Factory,
	backupBucket *extensionsv1alpha1.BackupBucket,
	storageAccountKey string,
) (string, error) {
	var (
		resourceGroupName   = backupBucket.Name
		backupBucketNameSha = utils.ComputeSHA256Hex([]byte(backupBucket.Name))
		storageAccountName  = fmt.Sprintf("bkp%s", backupBucketNameSha[:15])
	)
	storageAccountClient, err := factory.StorageAccount()
	if err != nil {
		return "", err
	}
	return storageAccountClient.RotateKey(ctx, resourceGroupName, storageAccountName, storageAccountKey)
}
