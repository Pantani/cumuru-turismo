import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { validateRotation } from "./PasswordRotationForm";
import AuthenticatedPage from "../../pages/AuthenticatedPage";
import { AuthError, type AuthClient } from "../../shared/api/auth-client";
import { AuthSessionProvider } from "../../shared/auth/AuthSession";
import { testSession } from "../../test/session";

afterEach(cleanup);

const provisional = testSession(["stays:write"], true);
const rotated = testSession(["stays:write"], false);

function renderRotation(authClient: AuthClient) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0 } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <AuthSessionProvider authClient={authClient}>
        <AuthenticatedPage />
      </AuthSessionProvider>
    </QueryClientProvider>,
  );
}

async function signInWithProvisionalSecret(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText("E-mail"), "operador@cumuru.local");
  await user.type(screen.getByLabelText("Senha"), "senha-provisoria");
  await user.click(screen.getByRole("button", { name: /entrar/i }));
}

describe("validateRotation", () => {
  it("rejects a new secret equal to the current one", () => {
    const issues = validateRotation(
      "senha-provisoria",
      "senha-provisoria",
      "senha-provisoria",
    );
    expect(issues.next).toContain("diferente da atual");
  });

  it("rejects a confirmation that does not match", () => {
    const issues = validateRotation(
      "senha-provisoria",
      "senha-definitiva",
      "senha-diferente",
    );
    expect(issues.confirmation).toContain("não confere");
  });

  it("accepts a long enough and distinct secret", () => {
    expect(
      validateRotation("senha-provisoria", "senha-definitiva", "senha-definitiva"),
    ).toEqual({});
  });
});

describe("PasswordRotationForm", () => {
  // A provisional credential authorizes nothing, so the operator must land on
  // the rotation screen rather than on the workspace.
  it("replaces the workspace while the secret is provisional", async () => {
    const user = userEvent.setup();
    renderRotation({
      login: async () => provisional,
      describe: async () => provisional,
      rotatePassword: async () => undefined,
      logout: async () => undefined,
    });
    await signInWithProvisionalSecret(user);
    expect(
      await screen.findByRole("heading", { name: "Defina uma senha própria" }),
    ).toBeTruthy();
  });

  it("signs in again with the new secret after rotating", async () => {
    const user = userEvent.setup();
    const rotatePassword = vi.fn(async () => undefined);
    let signedIn = false;
    renderRotation({
      login: async () => {
        const session = signedIn ? rotated : provisional;
        signedIn = true;
        return session;
      },
      describe: async () => rotated,
      rotatePassword,
      logout: async () => undefined,
    });
    await signInWithProvisionalSecret(user);
    await screen.findByRole("heading", { name: "Defina uma senha própria" });

    await user.type(screen.getByLabelText("Senha atual"), "senha-provisoria");
    await user.type(screen.getByLabelText("Nova senha"), "senha-definitiva");
    await user.type(
      screen.getByLabelText("Confirme a nova senha"),
      "senha-definitiva",
    );
    await user.click(screen.getByRole("button", { name: /trocar senha/i }));

    await waitFor(() => {
      expect(rotatePassword).toHaveBeenCalledWith(
        "cms_test-session-token",
        "senha-provisoria",
        "senha-definitiva",
      );
    });
    expect(
      await screen.findByRole("heading", { name: "Área da hospedagem" }),
    ).toBeTruthy();
  });

  // FieldError used to live inside the <label htmlFor>, so once an issue was
  // reported the input's accessible name absorbed the error text too — a
  // screen reader would read label and error as one run-on phrase instead of
  // announcing the error separately through aria-describedby. This proves the
  // accessible name stays exactly the label, both before and after an issue
  // is reported.
  it("keeps each password field's accessible name free of the error text", async () => {
    const user = userEvent.setup();
    renderRotation({
      login: async () => provisional,
      describe: async () => provisional,
      rotatePassword: async () => {
        throw new AuthError(422, "A nova senha precisa ter ao menos 12 caracteres.");
      },
      logout: async () => undefined,
    });
    await signInWithProvisionalSecret(user);
    await screen.findByRole("heading", { name: "Defina uma senha própria" });

    const currentField = screen.getByLabelText("Senha atual");
    const nextField = screen.getByLabelText("Nova senha");
    const confirmationField = screen.getByLabelText("Confirme a nova senha");
    expect(currentField).toHaveAccessibleName("Senha atual");
    expect(nextField).toHaveAccessibleName("Nova senha");
    expect(confirmationField).toHaveAccessibleName("Confirme a nova senha");

    await user.type(currentField, "senha-provisoria");
    await user.type(nextField, "senha-definitiva");
    await user.type(confirmationField, "senha-definitiva");
    await user.click(screen.getByRole("button", { name: /trocar senha/i }));
    await screen.findByRole("alert");

    // The submission failure does not touch these three fields' own issues,
    // so exercise the validation-issue path directly: clear the confirmation
    // to provoke a mismatch and confirm the name is still just the label.
    await user.clear(confirmationField);
    await user.type(confirmationField, "outra-coisa-qualquer");
    await user.click(screen.getByRole("button", { name: /trocar senha/i }));
    await waitFor(() => {
      expect(screen.getByText("A confirmação não confere com a nova senha.")).toBeTruthy();
    });
    expect(screen.getByLabelText("Senha atual")).toHaveAccessibleName("Senha atual");
    expect(screen.getByLabelText("Nova senha")).toHaveAccessibleName("Nova senha");
    expect(
      screen.getByLabelText("Confirme a nova senha"),
    ).toHaveAccessibleName("Confirme a nova senha");
  });

  it("reports a rejected secret without leaving the screen", async () => {
    const user = userEvent.setup();
    renderRotation({
      login: async () => provisional,
      describe: async () => provisional,
      rotatePassword: async () => {
        throw new AuthError(422, "A nova senha precisa ter ao menos 12 caracteres.");
      },
      logout: async () => undefined,
    });
    await signInWithProvisionalSecret(user);
    await screen.findByRole("heading", { name: "Defina uma senha própria" });

    await user.type(screen.getByLabelText("Senha atual"), "senha-provisoria");
    await user.type(screen.getByLabelText("Nova senha"), "senha-definitiva");
    await user.type(
      screen.getByLabelText("Confirme a nova senha"),
      "senha-definitiva",
    );
    await user.click(screen.getByRole("button", { name: /trocar senha/i }));

    expect(await screen.findByRole("alert")).toBeTruthy();
    expect(
      screen.getByRole("heading", { name: "Defina uma senha própria" }),
    ).toBeTruthy();
  });
});
