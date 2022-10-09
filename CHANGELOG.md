### Update 1008

Prompt Build Assist added, which finetuned from a GPT Neo2.7B model for adding rich and varied modifiers of art style / details to help creators to complete a Stabile Diffusion prompt subject with only a few words.
Training set: 200K prompts selected and preprocessed from krea-ai open-prompts.
Model download: [https://huggingface.co/huoju/gptneoforsdprompt](https://huggingface.co/huoju/gptneoforsdprompt)

Usage:

- only add style or detail modifiers after your prompt
!build + your prompt (could just be a subject with very few words)

-  complete the subject & add modifiers
!build + prompt + ...

### Update 1007

- Has upgraded to support diffusers v 0.4.0
- Support negative weight for prompt, use | to separate your negative prompt, add :-1 at the end.

For example:  !dream Bouquet of Roses, Dutch golden age art | red rose, yellow rose :-1 |

### Update 0922

[New Variant] button added, for regenerating the same prompt with different seed in a easier way.
