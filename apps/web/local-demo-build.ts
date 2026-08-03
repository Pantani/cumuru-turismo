interface LocalDemoBuildEnvironment {
  VITE_LOCAL_DEMO_IDENTITY?: string;
  VITE_LOCAL_DEMO_MODE?: string;
}

export function validateLocalDemoBuild(
  environment: LocalDemoBuildEnvironment,
) {
  const mode = String(environment.VITE_LOCAL_DEMO_MODE ?? "").trim();
  const identity = String(
    environment.VITE_LOCAL_DEMO_IDENTITY ?? "",
  ).trim();
  if (!new Set(["", "false", "true"]).has(mode)) {
    throw new Error("VITE_LOCAL_DEMO_MODE must be true or false");
  }
  if (mode === "true") {
    if (identity !== "") {
      return;
    }
    throw new Error(
      "VITE_LOCAL_DEMO_IDENTITY is required when local demo mode is enabled",
    );
  }
  if (identity !== "") {
    throw new Error(
      "VITE_LOCAL_DEMO_IDENTITY requires local demo mode",
    );
  }
}
