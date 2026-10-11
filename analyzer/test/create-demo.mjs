import { createProject } from './fixture.mjs';
if (!process.argv[2]) throw new Error('Provide the fixture directory.');
await createProject(process.argv[2], { dynamic: process.argv.includes('--dynamic') });
