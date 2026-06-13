export interface AppBindings {
  Submit(arg1: string): Promise<void>;
  ExtraInDTSOnly(arg1: string): Promise<void>;
}

export const app = {
  Submit: async (_msg: string) => {},
};
