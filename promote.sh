#!/bin/bash

echo "promoting the new version ${VERSION} to downstream repositories"

#jx step create pr go --name github.com/jenkins-x/jx-admin --version ${VERSION} --build "make build" --repo https://github.com/jenkins-x/jxl.git

#jx step create pr regex --regex "^(?m)\s+name: helmboot\s+version: \"(.*)\"$"  --version ${VERSION} --files alpha/plugins.yml --repo https://github.com/jenkins-x/jxl.git

#jx step create pr regex --regex "^jxRemoteVersion:\s+\"(.*)\"$"  --version ${VERSION} --files charts/jx-app-cb-remote/values.yaml --repo https://github.com/cloudbees/jx-app-cb-remote.git

#jx step create pr regex --regex "/cloudbees-jx-admin-plugin/plugin/(.*)/"  --version ${VERSION} --files README.md --repo https://github.com/cloudbees/jx-app-cb-remote.git

#jx step create pr regex --regex "/cloudbees-jx-admin-plugin/plugin/(.*)/"  --version ${VERSION} --files remote-environments/README.md --repo https://github.com/cloudbees/jx-previews.git
