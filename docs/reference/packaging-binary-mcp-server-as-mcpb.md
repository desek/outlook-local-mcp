# Packaging a Binary MCP Server as an MCPB Extension

A complete guide from raw binary to one-click install in Claude Desktop.

## Prerequisites

- Your compiled binary (the MCP server), built for your target platform(s)
- Node.js 18+ (for the `mcpb` CLI)
- Claude Desktop (latest version, for testing)

## Step 1: Install the MCPB CLI

```bash
npm install -g @anthropic-ai/mcpb
```

Verify the install:

```bash
mcpb --version
```

## Step 2: Scaffold your extension

Create a directory for your extension and initialize it:

```bash
mkdir my-extension
cd my-extension
mcpb init
```

The interactive prompts will ask for a name, description, author, and server type. When asked about the server type, choose **binary**. This generates a starter `manifest.json` you will customize in the next step.

## Step 3: Write the manifest

Open `manifest.json` and edit it to match your binary. Here is a complete example for a binary server with one tool:

```json
{
  "mcpb_version": "0.1",
  "name": "my-awesome-tool",
  "version": "1.0.0",
  "display_name": "My Awesome Tool",
  "description": "A short, clear sentence about what your extension does.",
  "author": {
    "name": "Your Name"
  },
  "server": {
    "transport": "stdio",
    "type": "binary",
    "binary": {
      "path": {
        "darwin_arm64": "bin/my-server-darwin-arm64",
        "darwin_x64": "bin/my-server-darwin-x64",
        "win32_x64": "bin/my-server-win32-x64.exe"
      }
    }
  },
  "tools": [
    {
      "name": "do_something",
      "description": "Describe what this tool does so Claude knows when to use it."
    }
  ],
  "compatibility": {
    "platforms": ["darwin_arm64", "darwin_x64", "win32_x64"]
  },
  "user_config": {}
}
```

Key things to note:

- **`server.binary.path`** maps each platform to the relative path of the correct binary inside the bundle.
- **`transport`** must be `"stdio"` for local servers.
- **`tools`** should list every tool your server exposes. Claude Desktop shows these to users during install.
- If your server needs user-provided config (like an API key), add fields under `user_config`. For example:

```json
"user_config": {
  "api_key": {
    "type": "string",
    "title": "API Key",
    "description": "Your API key from example.com",
    "required": true,
    "sensitive": true
  }
}
```

Setting `"sensitive": true` tells Claude Desktop to store the value in the OS keychain.

## Step 4: Add your binaries

Create a `bin/` directory and copy in the compiled binaries for each platform you support:

```bash
mkdir bin
cp /path/to/my-server-darwin-arm64  bin/my-server-darwin-arm64
cp /path/to/my-server-darwin-x64    bin/my-server-darwin-x64
cp /path/to/my-server-win32-x64.exe bin/my-server-win32-x64.exe
```

Make sure the Unix binaries are executable:

```bash
chmod +x bin/my-server-darwin-arm64
chmod +x bin/my-server-darwin-x64
```

Your directory should now look like this:

```
my-extension/
├── manifest.json
├── bin/
│   ├── my-server-darwin-arm64
│   ├── my-server-darwin-x64
│   └── my-server-win32-x64.exe
├── icon.png              (optional, 512x512 recommended)
└── README.md             (optional but strongly recommended)
```

## Step 5: Validate

Run the validator to catch manifest errors before packaging:

```bash
mcpb validate manifest.json
```

Fix any reported issues (missing required fields, bad paths, etc.) before continuing.

## Step 6: Pack

Build the `.mcpb` bundle:

```bash
mcpb pack .
```

This produces a file like `my-awesome-tool.mcpb` in the current directory. You can inspect it with:

```bash
mcpb info my-awesome-tool.mcpb
```

This shows the parsed manifest, included files, and bundle size, which is useful for a final sanity check.

## Step 7: Sign (optional but recommended)

Signing your bundle lets Claude Desktop verify its integrity. For development and testing, a self-signed certificate works:

```bash
mcpb sign my-awesome-tool.mcpb --self-signed
```

For production distribution, use a real certificate:

```bash
mcpb sign my-awesome-tool.mcpb --cert cert.pem --key key.pem
```

## Step 8: Test locally

1. Open Claude Desktop.
2. Go to **Settings > Extensions**.
3. Click **"Install Extension..."** and select your `.mcpb` file.
4. If your manifest includes `user_config` fields, Claude Desktop will prompt for them now.
5. Restart Claude Desktop if prompted.
6. Open a new conversation and ask Claude to use one of your tools:

> "Use the do_something tool to ..."

7. Check the extension logs under **Settings > Extensions > (your extension)** if anything goes wrong.

## Step 9: Distribute

You have two distribution paths.

### Option A: Share the file directly

Send the `.mcpb` file to users (via your website, GitHub releases, email, etc.). They install it manually through **Settings > Extensions > Install Extension...** in Claude Desktop. This is great for internal tools, small teams, or early beta testing.

### Option B: Submit to the Anthropic Directory

For public discovery (the "app store" experience), submit your extension to the Anthropic Directory:

1. Make sure you have a thorough `README.md` with at least 3 usage examples.
2. Add a privacy policy URL if your extension accesses external services or sends data off-device.
3. Fill out the submission form at: **https://forms.gle/tyiAZvch1kDADKoP9**
4. Anthropic reviews the extension. Common reasons for delay or rejection include incomplete documentation and missing privacy policies.
5. Once approved, users can find and install your extension from the **"Browse extensions"** page inside Claude Desktop.

## How the user experience looks

Once your extension is published (or shared), this is what a user does:

1. Open Claude Desktop and go to **Settings > Extensions**.
2. Either browse the directory and click **Install**, or click **"Install Extension..."** and select the `.mcpb` file.
3. Enter any required configuration (API keys, workspace paths, etc.).
4. Start a conversation and ask Claude to do something your tool handles.
5. Claude calls your tool, the binary runs locally on the user's machine, and results appear in the chat.

No terminal, no `npm install`, no config files. One click.

## Quick reference: useful commands

| Command                          | What it does                              |
| -------------------------------- | ----------------------------------------- |
| `mcpb init`                      | Scaffold a new extension interactively    |
| `mcpb validate manifest.json`    | Check your manifest for errors            |
| `mcpb pack .`                    | Bundle everything into a `.mcpb` file     |
| `mcpb sign file.mcpb`            | Sign the bundle for integrity checks      |
| `mcpb info file.mcpb`            | Inspect the contents of a packed bundle   |

## Further reading

- MCPB specification and CLI docs: [github.com/modelcontextprotocol/mcpb](https://github.com/modelcontextprotocol/mcpb)
- Manifest field reference: [MANIFEST.md](https://github.com/modelcontextprotocol/mcpb/blob/main/MANIFEST.md)
- Submission guide: [Claude Help Center](https://support.claude.com/en/articles/12922832-local-mcp-server-submission-guide)
- Building guide: [Claude Help Center](https://support.claude.com/en/articles/12922929-building-desktop-extensions-with-mcpb)
