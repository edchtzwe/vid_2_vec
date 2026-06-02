const NODE_ENV = process.env.NODE_ENV || "development";
const isDev = NODE_ENV === "development";

const workers = [
  "worker-analyzer",
  "worker-embedder",
  "worker-state",
  "worker-uploader",
];

const servers = ["api"];

function makeApp(name, { singleInstance = false } = {}) {
  const base = {
    name,
    cwd: "/app",
    autorestart: true,
    watch: false,
    max_memory_restart: "512M",
    interpreter: "none",
    env: {
      NODE_ENV,
    },
  };

  if (isDev) {
    return {
      ...base,
      script: "go",
      args: `run cmd/${name}/main.go`,
      instances: 1,
    };
  }

  return {
    ...base,
    script: `./bin/${name}`,
    instances: singleInstance ? 1 : 2,
    exec_mode: "fork",
  };
}

module.exports = {
  apps: [
    ...servers.map((name) => makeApp(name, { singleInstance: true })),
    ...workers.map((name) => makeApp(name)),
  ],
};
