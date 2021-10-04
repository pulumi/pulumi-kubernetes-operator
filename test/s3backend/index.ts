import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const bucket = new aws.s3.Bucket(`k8s-operator-s3-backend-bucket`,
    {bucket: "pulumi-k8s-operator-s3-backend-bucket"});

export const bucketName = bucket.id;

const key = new aws.kms.Key("kms-key", {deletionWindowInDays: 10, description: 'backend key'});

export const kmsKey = key.arn;
