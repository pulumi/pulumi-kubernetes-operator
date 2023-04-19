import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const cfg = new pulumi.Config("aws");
// We will induce a change in region to test failures.
const region = cfg.require("region");

const prov = new aws.Provider("provider", {region: region as aws.Region});
const names = [];
for (let i = 0; i < 2; i++) {
    // Create an AWS resource (S3 Bucket)
    const bucket = new aws.s3.Bucket(`my-bucket-${i}`, {
        tags: {"region": region},
    }, {
        provider: prov,
    });
    names.push(bucket.id);
}

// Export the name of the buckets
export const bucketNames = names;

// Export the bucket names as a secret
export const bucketsAsSecrets = pulumi.secret(bucketNames);
