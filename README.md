## PatchBot Technology Stack

| Component            | Technology              | Purpose                                                                                               |
| -------------------- | ----------------------- | ----------------------------------------------------------------------------------------------------- |
| Programming Language | TypeScript              | Implements the core PatchBot logic and risk engine                                                    |
| Runtime              | Node.js                 | Runs PatchBot locally on developers' machines                                                         |
| CLI Framework        | Commander.js            | Provides commands such as `patchbot scan`                                                             |
| Dependency Analysis  | npm Arborist            | Analyzes dependency trees and identifies direct, transitive, production, and development dependencies |
| Vulnerability Data   | OSV API                 | Retrieves known vulnerability information for installed packages                                      |
| Source Code Analysis | ts-morph                | Analyzes TypeScript/JavaScript source code, imports, and package usage                                |
| Risk Engine          | Custom TypeScript Rules | Combines vulnerability, dependency, and code context to prioritize security findings                  |
| AI Explanation       | LLM API (Optional)      | Converts technical findings into developer-friendly explanations                                      |
| Terminal Interface   | Chalk + Ora             | Provides readable, colored CLI output and progress indicators                                         |
| Testing              | Vitest                  | Tests PatchBot's scanning, analysis, and risk-classification logic                                    |

### Core Context-Aware Pipeline

OSV API
→ Identifies known vulnerabilities

npm Arborist
→ Determines where vulnerable dependencies exist in the project's dependency tree

ts-morph
→ Examines how dependencies are imported and used by the application

Custom TypeScript Risk Engine
→ Combines the collected evidence to assign a practical risk priority

Optional LLM
→ Explains the evidence and risk classification in developer-friendly language
