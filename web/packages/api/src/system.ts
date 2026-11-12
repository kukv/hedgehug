import type { SystemPort } from "@hedgehug/core";
import { getSystem } from "./generated";

export function createSystemPort(): SystemPort {
  return {
    async getSystem() {
      const body = await getSystem();
      return { version: body.version };
    },
  };
}
