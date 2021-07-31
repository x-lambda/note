import csv
import random
import math
import operator


def load_dataset(filename, split, training_set=[], test_set=[]):
    """加载数据集
    :param filename:
    :param split:
    :param training_set:
    :param test_set:
    :return:
    """
    with open(filename, 'r') as csv_file:
        lines = csv.reader(csv_file)
        dataset = list(lines)

        for x in range(len(dataset) - 1):
            for y in range(4):
                dataset[x][y] = float(dataset[x][y])

                if random.random() < split:
                    # 将数据随机划分为训练数据和测试数据
                    training_set.append(dataset[x])
                else:
                    test_set.append(dataset[x])


def euclidean_distance(p1, p2, length):
    """计算点之间的距离，多维度的

    s = math.sqrt((p1[0] - p2[0])^2 + (p1[1] - p2[1])^2 + (p1[2] - p2[2])^2 + ... + (p1[length-1] - p2[length-1])^2)
    :param p1:
    :param p2:
    :param length: 维度
    :return:
    """
    distance = 0

    for x in range(length):
        distance += pow((p1[x] - p2[x]), 2)

    return math.sqrt(distance)


def get_neighbors(train_set, p, k):
    distance = []
    length = len(p) - 1

    for x in range(len(train_set)):
        dist = euclidean_distance(p, train_set[x], length)
        # 获取目标点到训练数据集中的点的距离
        distance.append((train_set[x], dist))

    # 对所有的距离进行排序
    distance.sort(key=operator.itemgetter(1))
    # 获取到目标点最近的k个点
    neighbors = []
    for x in range(k):
        neighbors.append(distance[x][0])

    # TODO 这里可以优化使用最小堆
    return neighbors


def get_response(neighbors):
    """得到k个近邻的分类中最多的那个
    :param neighbors:
    :return:
    """
    class_votes = {}
    for x in range(len(neighbors)):
        response = neighbors[x][-1]
        if response in class_votes:
            class_votes[response] += 1
        else:
            class_votes[response] = 1

    sorted_votes = sorted(class_votes.items(), key=operator.itemgetter(1), reverse=True)

    # 返回票数最多的类别
    return sorted_votes[0][0]


def get_accuracy(test_set, predictions):
    """计算预测的准确率
    :param test_set:
    :param predictions:
    :return:
    """
    correct = 0
    for x in range(len(test_set)):
        if test_set[x][-1] == predictions[x]:
            correct += 1

    return (correct / float(len(test_set))) * 100.0


def main():
    train_set = []
    test_set = []
    split = 0.67     # 数据集中的 2/3 作为训练集， 1/3 作为测试集

    # iris data 下载地址 http://archive.ics.uci.edu/ml/machine-learning-databases/iris/iris.data
    # iris data数据有4个维度
    # https://zhuanlan.zhihu.com/p/75127982
    filename = 'iris.data'
    load_dataset(filename, split, train_set, test_set)

    predictions = []
    k = 3
    for x in range(len(test_set)):
        neighbors = get_neighbors(train_set, test_set[x], k)
        result = get_response(neighbors)
        predictions.append(result)
        print('predicted=' + repr(result) + ', actual=' + repr(test_set[x][-1]))

    print('predictions: ' + repr(predictions))
    accuracy = get_accuracy(test_set, predictions)
    print('Accuracy: ' + repr(accuracy) + '%')


if __name__ == "__main__":
    main()

