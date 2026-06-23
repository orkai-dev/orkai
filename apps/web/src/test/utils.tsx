import { QueryClientProvider } from "@tanstack/react-query";
import {
  type RenderHookOptions,
  type RenderHookResult,
  type RenderOptions,
  render,
  renderHook,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement, ReactNode } from "react";
import { createTestQueryClient } from "./query-client";

export * from "@testing-library/react";
export { userEvent };

type ProviderOptions = {
  queryClient?: ReturnType<typeof createTestQueryClient>;
};

function createWrapper(queryClient: ReturnType<typeof createTestQueryClient>) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

export function renderWithProviders(
  ui: ReactElement,
  options?: Omit<RenderOptions, "wrapper"> & ProviderOptions,
) {
  const { queryClient = createTestQueryClient(), ...renderOptions } = options ?? {};
  const user = userEvent.setup();

  return {
    user,
    queryClient,
    ...render(ui, { wrapper: createWrapper(queryClient), ...renderOptions }),
  };
}

export function renderHookWithProviders<Result, Props>(
  hook: (props: Props) => Result,
  options?: Omit<RenderHookOptions<Props>, "wrapper"> & ProviderOptions,
): RenderHookResult<Result, Props> & { queryClient: ReturnType<typeof createTestQueryClient> } {
  const { queryClient = createTestQueryClient(), ...hookOptions } = options ?? {};

  const result = renderHook(hook, {
    wrapper: createWrapper(queryClient),
    ...hookOptions,
  });

  return { ...result, queryClient };
}
