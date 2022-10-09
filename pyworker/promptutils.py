from enum import Enum

class ParseStatus(Enum):
    NoSeg = 1
    StartSeg = 2
    Weight = 3
    EndSeg = 4
def promptToSegs(prompt):
    pstatus = ParseStatus.NoSeg
    segs = []
    pnosegstartidx = 0
    psegstartidx = 0 #start
    pweightstartidx = len(prompt) #end
    try:
        for idx, char in enumerate(prompt):
            if char == '|':
                if pstatus == ParseStatus.NoSeg:
                    pstatus = ParseStatus.StartSeg        
                    psegstartidx = idx + 1
                elif pstatus == ParseStatus.Weight:
                    if  psegstartidx-1 - pnosegstartidx > 0:
                        segs.append({"seg":prompt[pnosegstartidx : psegstartidx-1].strip(), "weight":0})
                    segs.append({"seg":prompt[psegstartidx:pweightstartidx-1].strip(), "weight":int(prompt[pweightstartidx:idx])})
                    #reset index and pstatus
                    psegstartidx= 0
                    pweightstartidx = len(prompt)
                    pnosegstartidx = idx + 1
                    pstatus = ParseStatus.NoSeg
                elif pstatus == ParseStatus.StartSeg:
                    psegstartidx = idx + 1
            elif char == ':':
                if pstatus == ParseStatus.StartSeg:
                    if idx+1 < len(prompt) and (prompt[idx+1] == '-' or  prompt[idx+1].isnumeric() == True):
                        pstatus = ParseStatus.Weight
                        pweightstartidx = idx + 1
    except Exception as e:
        print(e)
    if len(segs)==0:
        segs.append({"seg":prompt, "weight":0})
    return segs


def gptoutputToPrompt(output):
    outputtag = "<|modeloutput|>"
    i = output.find(outputtag)
    output= output[i+len(outputtag) :len(output)]
    i = output.find(outputtag)
    if i > 0:
        output = output[0:i]
    return output
