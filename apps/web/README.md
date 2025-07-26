# devex-site

## Overview

`devex-site` is a Next.js project designed to provide a seamless developer
experience. This project uses `pnpm` for package management and `mise` for
managing Linux packages.

## Getting Started

### Prerequisites

- Node.js (v18.x or later)
- `pnpm` (v9.x or later)
- `mise` (v2024.12.x or later)

### Installation

1. **Clone the repository:**

   ```bash
   git clone https://github.com/jameswlane/devex-site.git
   cd devex-site
   ```

2. **Install system dependencies using `mise`:**

   ```bash
   mise install
   ```

3. **Install dependencies using `pnpm`:**
   ```bash
   pnpm install
   ```

### Running the Development Server

To start the development server, run:

```bash
pnpm dev
```

Open [http://localhost:3000](http://localhost:3000) with your browser to see the
result.

### Building for Production

To create an optimized production build, run:

```bash
pnpm build
```

### Running in Production

To run the project in production mode, run:

```bash
pnpm start
```

## Learn More

To learn more about Next.js, `pnpm`, and `mise`, take a look at the following
resources:

- [Next.js Documentation](https://nextjs.org/)
- [pnpm Documentation](https://pnpm.io/)
- [mise Documentation](https://mise.jdx.dev/)

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## License

This project is licensed under the MIT License.
