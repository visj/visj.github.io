import fs from 'fs/promises';
import path from 'path';
import { createMarkdownExit } from 'markdown-exit';
import Shiki from '@shikijs/markdown-exit';
import matter from 'gray-matter';

const CONTENT_DIR = './content';
const DIST_DIR = './dist';
const TEMPLATE_PATH = './src/template.html';

async function build() {
  // 1. Initialize markdown-exit using the factory helper
  const md = createMarkdownExit();

  // 2. Add Shiki plugin for syntax highlighting
  // This will automatically handle tokenizing code blocks asynchronously
  md.use(Shiki({
    themes: {
      light: 'github-light',
      dark: 'github-dark',
    }
  }));

  // 3. Prepare directories and load the HTML template
  await fs.mkdir(DIST_DIR, { recursive: true });
  const template = await fs.readFile(TEMPLATE_PATH, 'utf-8');
  
  // Copy over the global CSS file
  await fs.copyFile('./src/style.css', path.join(DIST_DIR, 'style.css'));

  // 4. Recursive function to read all markdown files and mirror the folder structure
  async function processDirectory(dir) {
    const entries = await fs.readdir(dir, { withFileTypes: true });

    for (const entry of entries) {
      const fullPath = path.join(dir, entry.name);
      
      // Get the path relative to the content folder (e.g., "dev/my-post.md")
      const relativePath = path.relative(CONTENT_DIR, fullPath);

      if (entry.isDirectory()) {
        // If it's a folder, create the equivalent folder in /dist
        const destDir = path.join(DIST_DIR, relativePath);
        await fs.mkdir(destDir, { recursive: true });
        await processDirectory(fullPath);
      } else if (entry.name.endsWith('.md')) {
        // If it's a markdown file, figure out the destination HTML path
        const destPath = path.join(DIST_DIR, relativePath).replace(/\.md$/, '.html');
        const fileContent = await fs.readFile(fullPath, 'utf-8');
        
        // Parse YAML frontmatter (like title) and the raw markdown body
        const { data, content } = matter(fileContent);
        const title = data.title || 'Vilhelm.se';

        // Render markdown asynchronously (required for the Shiki plugin to work)
        const htmlContent = await md.renderAsync(content);

        // Calculate correct relative path for the CSS file based on folder depth
        const depth = relativePath.split(path.sep).length - 1;
        const cssPath = depth === 0 ? './style.css' : '../'.repeat(depth) + 'style.css';

        // Inject everything into the template shell
        const finalHtml = template
          .replace('{{TITLE}}', title)
          .replace('{{CSS_PATH}}', cssPath)
          .replace('{{CONTENT}}', htmlContent);

        // Write the compiled HTML file
        await fs.writeFile(destPath, finalHtml);
        console.log(`Built: ${destPath}`);
      }
    }
  }

  // Kick off the build process
  await processDirectory(CONTENT_DIR);
  console.log('✅ Build complete!');
}

build().catch(console.error);