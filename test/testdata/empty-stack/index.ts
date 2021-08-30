import * as pulumi from "@pulumi/pulumi";

console.log("Empty stack!");

const conf = new pulumi.Config("aws");
export const region = conf.require("region");

export const notSoSecret = pulumi.output("safe")
export const secretVal = pulumi.secret({
    "val" : "very secret"
});
