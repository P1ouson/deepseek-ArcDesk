import { app } from "./bridge";
import { getStoredCodeWorkspaceRoot, isUsableCodeWorkspaceRoot } from "./composerWorkspace";
import { isUsableWriteWorkspaceRoot } from "./writeWorkspace";

const WRITE_DOC_INJECT_MAX = 24_000;

export type WriteAgentContext = {
  writeFilePath?: string;
  writeWorkspaceRoot?: string;
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
  const codeRoot = getStoredCodeWorkspaceRoot();

  if (writeFile) {
    try {
      const body = await app.ReadWriteFile(writeFile);
      const clipped =
        body.length > WRITE_DOC_INJECT_MAX
          ? `${body.slice(0, WRITE_DOC_INJECT_MAX)}\n\n[…文稿已截断，共 ${body.length} 字符]`
          : body;
      blocks.push(
        `[写作区当前文稿]\n路径: ${writeFile}\n---\n${clipped}\n---`,
      );
    } catch {
      blocks.push(`[写作区当前文稿]\n路径: ${writeFile}\n（未能读取正文，请用户确认文件可访问）`);
    }
  } else if (writeRoot && isUsableWriteWorkspaceRoot(writeRoot)) {
    blocks.push(`[写作区保存位置]\n${writeRoot}`);
  }

  if (isUsableCodeWorkspaceRoot(codeRoot)) {
    blocks.push(
      `[代码区当前项目]\n路径: ${codeRoot}\n说明: 用户要求与代码项目联动时，请用 read_file / ls / grep 从此目录读取，不要猜测或使用旧缓存内容。`,
    );
  }

  if (blocks.length === 0) return { displayText, submitText };
  const prefix = blocks.join("\n\n");
  return {
    displayText,
    submitText: `${prefix}\n\n${submitText}`,
  };
}
