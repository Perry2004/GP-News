import {
	SESClient,
	SendEmailCommand,
	type SendEmailCommandOutput,
} from "@aws-sdk/client-ses";
import { render } from "@react-email/render";
import z from "zod";
import type { BriefingEmailData } from "./briefingSchema.js";
import { GpNewsEmailTemplate, renderBriefingAsText } from "./emailTemplate.js";

const envSchema = z.object({
	EMAIL_RECEIVERS: z
		.string()
		.min(1)
		.transform((str) => {
			const splitted = str.split(",");
			return splitted.map((s) => s.trim()).filter((s) => s.length > 0);
		}),
	EMAIL_SENDER: z.string().min(1),
	AWS_REGION: z.string().min(1),
});

const SUBJECT = "GP-News Briefing";

export async function sendBriefingEmail(
	briefing: BriefingEmailData,
	subject: string = SUBJECT,
): Promise<SendEmailCommandOutput[]> {
	const envVars = envSchema.parse(process.env);
	const EMAIL_RECEIVERS = envVars.EMAIL_RECEIVERS;
	const EMAIL_SENDER = envVars.EMAIL_SENDER;
	const SES_REGION = envVars.AWS_REGION;

	const sesClient = new SESClient({
		region: SES_REGION,
	});

	if (EMAIL_RECEIVERS.length === 0) {
		throw new Error("EMAIL_RECEIVERS is empty after parsing");
	}

	const htmlBody = await render(GpNewsEmailTemplate({ briefing }));
	const textBody = renderBriefingAsText(briefing);

	return Promise.all(
		EMAIL_RECEIVERS.map((receiver) => {
			return sesClient.send(
				new SendEmailCommand({
					Source: EMAIL_SENDER,
					Destination: {
						ToAddresses: [receiver],
					},
					Message: {
						Subject: {
							Data: `${subject}`,
							Charset: "UTF-8",
						},
						Body: {
							Html: {
								Data: htmlBody,
								Charset: "UTF-8",
							},
							Text: {
								Data: textBody,
								Charset: "UTF-8",
							},
						},
					},
				}),
			);
		}),
	);
}
