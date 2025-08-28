# go_fio_pull Infrastructure Setup


## Prerequisites
- Azure CLI installed and authenticated
- GitHub CLI (`gh`) installed and authenticated
- jq installed (for JSON parsing)

## Setup Instructions

### 1. Configure Variables in `infra_base.sh`
Before running the script, open `infra_base.sh` and set the following variables:

```bash
LOCATION="westeurope"                # Azure region
PROJECT_NAME="gofiopull250828"        # Unique project name
PROJECT_NAME_NODASH="gofiopull250828" # Project name without dashes
REPO_NAME="viliamggh/go_fio_pull"     # GitHub repository in owner/repo format
```

### 2. Create an Empty GitHub Repository
Create an empty repository on GitHub with the name matching the value of `REPO_NAME` before running the script. Example:
- If `REPO_NAME="viliamggh/go_fio_pull"`, create `go_fio_pull` under your GitHub account `viliamggh`.

### 3. Login to Azure
Authenticate to Azure and select the correct subscription:

```sh
az login
```

### 4. Run the Script
After completing the above steps, execute the script:

```sh
bash infra_base.sh
```

This will provision the required Azure resources and set up GitHub variables for your workflows.