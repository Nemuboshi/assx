import * as vscode from 'vscode';
import { writeFile } from 'node:fs/promises';

async function until<T>(read: () => T | undefined, description: string): Promise<T> {
  const deadline = Date.now() + 15_000;
  while (Date.now() < deadline) {
    const value = read();
    if (value !== undefined) return value;
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  throw new Error(`Timed out waiting for ${description}`);
}

export async function run(): Promise<void> {
  try {
    await smokeTest();
    if (process.env.ASSX_TEST_MARKER) await writeFile(process.env.ASSX_TEST_MARKER, 'passed');
  } catch (error) {
    if (process.env.ASSX_TEST_MARKER) await writeFile(process.env.ASSX_TEST_MARKER, `failed: ${String(error)}`);
    throw error;
  }
}

async function smokeTest(): Promise<void> {
  const binary = process.env.ASSX_LSP_TEST_BINARY;
  if (!binary) throw new Error('ASSX_LSP_TEST_BINARY is required');
  await vscode.workspace.getConfiguration('assx').update('serverPath', binary, vscode.ConfigurationTarget.Global);
  const extension = vscode.extensions.all.find(candidate => candidate.packageJSON.name === 'assx-language-client');
  if (!extension) throw new Error('assx adapter was not loaded');

  const text = '[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n' +
    'Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\fs10\\fs20\\pos(1,2)}x\n';
  const document = await vscode.workspace.openTextDocument({ language: 'ass', content: text });
  await vscode.window.showTextDocument(document);
  await extension.activate();
  const diagnostics = await until(() => {
    const found = vscode.languages.getDiagnostics(document.uri);
    return found.some(item => item.code === 'ASS006') ? found : undefined;
  }, 'ASS006 diagnostic');
  const finding = diagnostics.find(item => item.code === 'ASS006');
  if (!finding) throw new Error('ASS006 was not published');

  const tag = new vscode.Position(2, document.lineAt(2).text.indexOf('\\pos') + 2);
  const hovers = await vscode.commands.executeCommand<vscode.Hover[]>('vscode.executeHoverProvider', document.uri, tag);
  if (!hovers?.length) throw new Error('hover was not available');
  const completions = await vscode.commands.executeCommand<vscode.CompletionList>('vscode.executeCompletionItemProvider', document.uri, tag);
  if (!completions?.items.length) throw new Error('completion was not available');

  const actions = await vscode.commands.executeCommand<(vscode.CodeAction | vscode.Command)[]>(
    'vscode.executeCodeActionProvider', document.uri, finding.range, vscode.CodeActionKind.QuickFix.value
  );
  const safe = actions?.find((item): item is vscode.CodeAction => 'edit' in item && !!item.edit);
  if (!safe?.edit || !(await vscode.workspace.applyEdit(safe.edit))) throw new Error('proven Quick Fix was not applied');
  if (document.getText().includes('\\fs10')) throw new Error('Quick Fix did not remove the redundant tag');

  await vscode.workspace.getConfiguration('assx').update('primaryRenderer', 'vsfiltermod', vscode.ConfigurationTarget.Global);
  await until(() => {
    const found = vscode.languages.getDiagnostics(document.uri);
    return found.some(item => String(item.message).includes('VSFilterMod')) ? found : undefined;
  }, 'renderer settings refresh');
}
