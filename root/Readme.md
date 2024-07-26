root/ (meta-repository)
├── environments/
│   ├── values-dev.yaml
│   ├── values-test.yaml
│   └── values-prod.yaml
├── kustomize/
│   ├── base/
│   │   ├── kustomization.yaml
│   │   ├── namespace.yaml
│   │   ├── network-policy.yaml
│   │   └── resource-quota.yaml
│   ├── dev/
│   │   └── kustomization.yaml
│   ├── test/
│   │   └── kustomization.yaml
│   └── prod/
│       └── kustomization.yaml
└── argo-cd/
    ├── app-of-apps.yaml
    └── apps/
        ├── app1.yaml
        ├── app2.yaml
        ├── app3.yaml
        ├── saas1.yaml
        └── saas2.yaml

# Separate repositories for each app (unchanged)
app1-repo/
├── Chart.yaml
├── values.yaml
└── templates/

app2-repo/
├── Chart.yaml
├── values.yaml
└── templates/

app3-repo/
├── Chart.yaml
├── values.yaml
└── templates/


This setup accommodates apps with Helm charts in different Git repositories while maintaining a DRY approach:

A meta-repository contains the ArgoCD configurations and environment-specific values.
Each app has its own repository with its Helm chart.
Environment-specific values are centralized in the meta-repository.
ArgoCD Applications reference both the app-specific repositories and the centralized value files.

To use this setup:

Set up your meta-repository with the structure provided.
Create separate repositories for each application's Helm chart.
Define your ArgoCD Applications in the meta-repository, pointing to the correct app repositories.
Set up your environment-specific value files in the meta-repository.
Apply the app-of-apps.yaml to your ArgoCD instance.

To deploy to different environments:

Update the global.env value in each Application's ArgoCD configuration.
ArgoCD will use this value to select the correct environment-specific value file.

This approach allows you to:

Manage apps from different repositories.
Keep a centralized configuration for environments.
Easily switch between environments by changing a single value.
Maintain separation of concerns between application code and deployment configuration.

Some additional considerations for this multi-repo setup:

Versioning: You might want to specify versions for each app's chart. This can be done in the ArgoCD Application definitions.
Secrets: Be cautious about storing sensitive information in the meta-repository. Consider using a secret management solution like Vault or Sealed Secrets.
CI/CD: Set up CI/CD pipelines for both the individual app repositories and the meta-repository.
Access Control: Ensure that ArgoCD has the necessary permissions to access all repositories.