import type {
	APIGatewayProxyEvent,
	APIGatewayProxyResult,
	Context,
} from "aws-lambda";
import "dotenv/config";
import { runBriefing } from "./main.js";

export async function handler(
	_event: APIGatewayProxyEvent,
	_context: Context,
): Promise<APIGatewayProxyResult> {
	try {
		const result = await runBriefing();

		return {
			statusCode: 200,
			body: JSON.stringify({
				message: "Briefing sent successfully",
				subject: result.subject,
			}),
		};
	} catch (error: unknown) {
		const message = error instanceof Error ? error.message : String(error);
		console.error(`Error in Lambda handler: ${message}`);
		return {
			statusCode: 500,
			body: JSON.stringify({
				message: "Failed to process briefing",
				error: message,
			}),
		};
	}
}
