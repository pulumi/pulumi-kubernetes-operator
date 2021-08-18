import * as pulumi from "@pulumi/pulumi";

console.log("Empty stack!");

export const notSoSecret = pulumi.output("safe")
export const secretVal = pulumi.secret({
    "val" : "very secret"
});
