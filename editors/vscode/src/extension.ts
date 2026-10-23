import * as vscode from 'vscode';
import { LanguageClient, TransportKind, type LanguageClientOptions, type ServerOptions } from 'vscode-languageclient/node';
import { serverSettings } from './settings.js';

let client: LanguageClient | undefined;

function settings() {
  const configuration = vscode.workspace.getConfiguration('assx');
  return serverSettings(key => configuration.get(key));
}

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  const executable = vscode.workspace.getConfiguration('assx').get<string>('serverPath', 'assx');
  const server: ServerOptions = { command: executable, args: ['lsp'], transport: TransportKind.stdio };
  const outputChannel = vscode.window.createOutputChannel('assx language server', { log: true });
  context.subscriptions.push(outputChannel);
  const options: LanguageClientOptions = {
    documentSelector: [
      { scheme: 'file', language: 'ass' }, { scheme: 'file', language: 'ssa' },
      { scheme: 'untitled', language: 'ass' }, { scheme: 'untitled', language: 'ssa' }
    ],
    initializationOptions: { assx: settings() },
    outputChannel
  };
  client = new LanguageClient('assx', 'assx', server, options);
  context.subscriptions.push(vscode.workspace.onDidChangeConfiguration(event => {
    if (!event.affectsConfiguration('assx')) return;
    if (event.affectsConfiguration('assx.serverPath')) {
      void vscode.window.showInformationMessage('Reload the window to use the new assx server path.');
      return;
    }
    void client?.sendNotification('workspace/didChangeConfiguration', { settings: { assx: settings() } });
  }));
  await client.start();
}

export async function deactivate(): Promise<void> {
  if (client) {
    await client.stop();
    client = undefined;
  }
}
