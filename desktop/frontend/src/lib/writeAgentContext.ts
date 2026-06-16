import { app } from "./bridge";
import {
  getStoredCodeWorkspaceRoot,
  isProjectLikeCodeWorkspaceRoot,
} from "./composerWorkspace";
import { isUsableWriteWorkspaceRoot } from "./writeWorkspace";
import { resolveWriteAgentWorkspaceRoot, type WriteAgentWorkspaceOptions } from "./writeTab";

const WRITE_DOC_INJECT_MAX = 24_000;

export type WriteAgentContext = {
  writeFilePath?: string;
  writeWorkspaceRoot?: string;
  workspaceOpts?: Omit<WriteAgentWorkspaceOptions, "codeWorkspaceRoot">;
};

/** Attach open document + code-project paths so the agent reads the right sources in 写作 mode. */
export async function enrichWriteModeSubmit(
  displayText: string,
  submitText: string,
  context: WriteAgentContext,
): Promise<{ displayText: string; submitText: string }> {
  const blocks: string[] = [];
  const writeFile = context.writeFilePath?.trim();
  const writeRoot = context.writeWorkspaceRoot?.trim();
  const codeRoot = resolveWriteAgentWorkspaceRoot({
    ...context.workspaceOpts,
    codeWorkspaceRoot: getStoredCodeWorkspaceRoot(),
  });

  if (writeFile) {
    try {
      const body = await app.ReadWriteFile(writeFile);
      const clipped =
        body.length > WRITE_DOC_INJECT_MAX
          ? `${body.slice(0, WRITE_DOC_INJECT_MAX)}\n\n[…文稿已截断，共 ${body.length} 字符]`
          : body;
      blocks.push(`[写作区当前文稿]\n路径: ${writeFile}\n---\n${clipped}\n---`);
    } catch {
      blocks.push(`[写作区当前文稿]\n路径: ${writeFile}\n（未能读取正文，请用户确认文件可访问）`);
    }
  } else if (writeRoot && isUsableWriteWorkspaceRoot(writeRoot)) {
    blocks.push(`[写作区保存位置]\n${writeRoot}`);
  }

  if (isProjectLikeCodeWorkspaceRoot(codeRoot)) {
    blocks.push(
      `[代码区当前项目]\n路径: ${codeRoot}\n` +
        "说明: 这是本次 Agent 的唯一代码工作区。所有 read_file / ls / grep / glob / bash 必须限定在此目录及其子路径；" +
        "禁止扫描 Desktop、用户主目录或其他未绑定的路径。用户问题涉及代码、模型、训练或项目事实时，从此目录读取，不要猜测。",
    );
  } else {
    blocks.push(
      "[代码区未绑定项目]\n" +
        "说明: 当前写作任务若需要读代码或项目数据，先用 ask 请用户在代码区打开具体项目目录，或让用户发送 / 绑定后再继续；不要对整个 Desktop 或磁盘根目录做 glob/ls。",
    );
  }

  if (blocks.length === 0) return { displayText, submitText };
  const prefix = blocks.join("\n\n");
  return {
    displayText,
    submitText: `${prefix}\n\n${submitText}`,
  };
}

export function hasLinkedCodeProject(context: WriteAgentContext): boolean {
  const codeRoot = resolveWriteAgentWorkspaceRoot({
    ...context.workspaceOpts,
    codeWorkspaceRoot: getStoredCodeWorkspaceRoot(),
  });
  return isProjectLikeCodeWorkspaceRoot(codeRoot);
}
