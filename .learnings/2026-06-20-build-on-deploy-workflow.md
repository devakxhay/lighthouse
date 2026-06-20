# Remote Pull and Build on Deploy Workflow

## Context
Originally, deployment required building application binaries locally and copying them to the host server (e.g. Raspberry Pi) via SCP. We transformed this architecture into a self-contained, remote Git pull and build-on-deploy model.

## Implementation Details
1. **App Type Build Automation**:
   - For **Spring Boot**, the build system automatically detects `./gradlew`, `./mvnw`, `pom.xml`, or `build.gradle` and builds the package with tests skipped.
   - For **Next.js**, it runs `npm install` and `npm run build`.
   - For **Go**, it builds the project binary via `go build -o <binary_path>`.
2. **Text File Busy Workaround**:
   - When building a Go binary on a running application, Linux raises a "text file busy" error if we attempt to overwrite the active running binary directly. We resolved this by unlinking/removing the file (`os.Remove`) before running `go build`.
3. **Zero Downtime Pattern**:
   - Pulling and building takes place *before* updating configurations and reloading systemd/Nginx services, preventing system shutdown or broken states if a build fails.
